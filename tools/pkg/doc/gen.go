package doc

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-tools/pkg/crd"
	"sigs.k8s.io/controller-tools/pkg/genall"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// Generator generates documentation from kinds.
type Generator struct {
	// WriteSchema tells the generator to output the JSON schema used by doc generation.
	WriteSchema bool `marker:",optional"`
	// Format specifies the output format for the documenation.  Supported types
	// are adoc (asciidoc) and md (markdown).  Defaults to adoc.
	Format string `marker:",optional"`

	// Depth specifies the number of levels to include in the main page.  Definitions
	// at greater depth are moved to their own pages
	Depth *int `marker:",optional"`

	template *template.Template
}

const (
	asciidocFormat = "adoc"
	markdownFormat = "md"
)

var _ genall.Generator = (*Generator)(nil)

func (Generator) RegisterMarkers(into *markers.Registry) error {
	return crd.Generator{}.RegisterMarkers(into)
}

func (g Generator) Generate(ctx *genall.GenerationContext) error {
	parser := &crd.Parser{
		Collector: ctx.Collector,
		Checker:   ctx.Checker,
	}

	if g.Format == "" {
		g.Format = asciidocFormat
	} else {
		g.Format = strings.ToLower(g.Format)
	}

	switch g.Format {
	case markdownFormat:
		g.template = markdownTemplate
	case asciidocFormat:
		g.template = asciidocTemplate
	default:
		return fmt.Errorf("Unsupported format specified: %s", g.Format)
	}

	if g.Depth == nil {
		defaultDepth := 0
		g.Depth = &defaultDepth
	}

	crd.AddKnownTypes(parser)
	for _, root := range ctx.Roots {
		parser.NeedPackage(root)
	}

	metav1Pkg := crd.FindMetav1(ctx.Roots)
	if metav1Pkg == nil {
		// no objects in the roots, since nothing imported metav1
		return nil
	}

	// TODO: allow selecting a specific object
	kubeKinds := crd.FindKubeKinds(parser, metav1Pkg)
	if len(kubeKinds) == 0 {
		// no objects in the roots
		return nil
	}

	groupToVersions := map[string][]schema.GroupVersion{}
	gvToKinds := map[schema.GroupVersion][]schema.GroupVersionKind{}
	kindToSchema := map[schema.GroupVersionKind]*apiext.JSONSchemaProps{}
	groupHasKinds := sets.NewString()
	for pkg, gv := range parser.GroupVersions {
		groupToVersions[gv.Group] = append(groupToVersions[gv.Group], gv)
		for groupKind := range kubeKinds {
			if gv.Group != groupKind.Group {
				continue
			}
			typeIdent := crd.TypeIdent{Package: pkg, Name: groupKind.Kind}
			typeInfo := parser.LookupType(pkg, groupKind.Kind)
			if typeInfo == nil {
				continue
			}
			groupHasKinds.Insert(gv.Group)
			gvToKinds[gv] = append(gvToKinds[gv], gv.WithKind(groupKind.Kind))
			parser.NeedSchemaFor(typeIdent)
			fullSchema := parser.Schemata[typeIdent]
			flattener := &flattener{
				Parser: parser,
			}
			fullSchema = *flattener.flattenSchema(fullSchema, pkg)
			fullSchema.ID = fmt.Sprintf("urn:%s/%s", typeIdent.Package.PkgPath, typeIdent.Name)
			kindToSchema[gv.WithKind(groupKind.Kind)] = &fullSchema

			if g.WriteSchema {
				schemaFileName := fmt.Sprintf("%s_%s_%s.yaml", gv.Group, groupKind.Kind, gv.Version)
				if err := ctx.WriteYAML(schemaFileName, fullSchema); err != nil {
					return err
				}
			}
		}
		gvkinds := gvToKinds[gv]
		sort.Slice(gvkinds, func(i, j int) bool { return strings.Compare(gvkinds[i].Kind, gvkinds[j].Kind) < 0 })
	}

	// filter groups with no kinds
	groups := groupHasKinds.List()

	// sort groups
	sort.Slice(groups, func(i, j int) bool { return strings.Compare(groups[i], groups[j]) > 0 })

	// sort versions for groups
	numericVersionMatcher := regexp.MustCompile("v([[:digit:]]+)([[:alpha:]]+[[:digit:]]+)*$")
	for _, group := range groups {
		gvs := groupToVersions[group]
		sort.Slice(gvs, func(i, j int) bool {
			// assume all versions start with "v"
			vi := gvs[i].Version
			vj := gvs[j].Version
			if vimatches := numericVersionMatcher.FindStringSubmatch(vi); vimatches[1] != "" && vimatches[2] == "" {
				if vjmatches := numericVersionMatcher.FindStringSubmatch(vj); vjmatches[1] != "" && vjmatches[2] == "" {
					// higher versions are sorted higher
					vi, _ := strconv.ParseInt(vimatches[1], 10, 64)
					vj, _ := strconv.ParseInt(vjmatches[1], 10, 64)
					return vj < vi
				}
				// numeric versions are always sorted higher than alpha, beta, etc. versions
				return true
			} else if numericVersionMatcher.MatchString(vj) {
				return false
			}
			// higher versions are sorted higher (beta sorts higher than alpha, etc.)
			// XXX: this doesn't account for version numbers greater than one digit
			return strings.Compare(vi, vj) > 0
		})
	}

	// Write out API
	apiTemplateData := struct {
		Groups        []string
		GroupVersions map[string][]schema.GroupVersion
		Kinds         map[schema.GroupVersion][]schema.GroupVersionKind
		Schemas       map[schema.GroupVersionKind]*apiext.JSONSchemaProps
	}{
		Groups:        groups,
		GroupVersions: groupToVersions,
		Kinds:         gvToKinds,
		Schemas:       kindToSchema,
	}
	if err := g.executeTemplate(ctx, "CRDSummary", apiTemplateData, fmt.Sprintf("CRDS.%s", g.Format)); err != nil {
		return err
	}

	// Calculate cross references
	locations, external := g.initializeCrossReferences(gvToKinds, kindToSchema)

	// Write out Kind
	for _, group := range groups {
		for _, gv := range groupToVersions[group] {
			for _, kind := range gvToKinds[gv] {
				kindSchema := kindToSchema[kind]
				templateData := struct {
					Root      *apiext.JSONSchemaProps
					Schema    *apiext.JSONSchemaProps
					Kind      schema.GroupVersionKind
					Locations map[string]string
					Seen      sets.String
					Depth     int
				}{
					Root:      kindSchema,
					Schema:    kindSchema,
					Kind:      kind,
					Locations: locations,
					Seen:      sets.NewString(),
					Depth:     *g.Depth,
				}
				if err := g.executeTemplate(ctx, "Kind", templateData, fileForKind(kind, g.Format)); err != nil {
					return errors.Wrapf(err, "error creating docs for %s", kind)
				}
			}
		}
	}

	for kind, externalSchemas := range external {
		rootSchema := kindToSchema[kind]
		for _, externalSchema := range externalSchemas {
			ref := rootSchema.Definitions[refName(*externalSchema.Ref)]
			id := ref.ID
			templateData := struct {
				Root      *apiext.JSONSchemaProps
				Schema    *apiext.JSONSchemaProps
				Name      string
				Kind      schema.GroupVersionKind
				Locations map[string]string
				Seen      sets.String
				Depth     int
			}{
				Root:      rootSchema,
				Schema:    &ref,
				Name:      path.Base(id),
				Kind:      kind,
				Locations: locations,
				Seen:      sets.NewString(),
				Depth:     0,
			}
			fileName := strings.Split(locations[id], "#")[0]
			if err := g.executeTemplate(ctx, "Type", templateData, fileName); err != nil {
				return errors.Wrapf(err, "error creating docs for %s", id)
			}
		}
	}

	return nil
}

func (g Generator) initializeCrossReferences(gvToKinds map[schema.GroupVersion][]schema.GroupVersionKind, schemasByKind map[schema.GroupVersionKind]*apiext.JSONSchemaProps) (map[string]string, map[schema.GroupVersionKind][]*apiext.JSONSchemaProps) {
	external := make(map[schema.GroupVersionKind][]*apiext.JSONSchemaProps)
	locations := make(map[string]string)
	seen := sets.NewString()
	for _, kinds := range gvToKinds {
		for _, kind := range kinds {
			schema, exists := schemasByKind[kind]
			if !exists {
				continue
			}
			kindLocations, kindExternal := g.traverseSchemaProperties(schema, schema, kind, seen, 1, *g.Depth)
			if kindExternal != nil {
				external[kind] = kindExternal
			}
			for key, val := range kindLocations {
				locations[key] = val
			}
		}
	}
	return locations, external
}

func (g Generator) traverseSchemaProperties(root, schema *apiext.JSONSchemaProps, kind schema.GroupVersionKind, seen sets.String, level, depth int) (map[string]string, []*apiext.JSONSchemaProps) {
	var props []*apiext.JSONSchemaProps
	locations := make(map[string]string)
	kindFile := fileForKind(kind, g.Format)
	refs := []apiext.JSONSchemaProps{}
	for _, dep := range schema.Properties {
		if dep.Ref == nil {
			continue
		}
		ref := root.Definitions[refName(*dep.Ref)]
		if differentPackage(root, &ref) {
			continue
		}
		if seen.Has(ref.ID) {
			continue
		}
		seen.Insert(ref.ID)
		if depth > 0 && level == depth {
			tmp := dep
			props = append(props, &tmp)
			locations[ref.ID] = fileForExternalReference(kind, *dep.Ref, g.Format)
		} else {
			locations[ref.ID] = fmt.Sprintf("%s#%s", kindFile, refName(*dep.Ref))
			refs = append(refs, ref)
		}
	}
	for _, ref := range refs {
		refLocations, externalProps := g.traverseSchemaProperties(root, &ref, kind, seen, level+1, depth)
		if externalProps != nil {
			props = append(props, externalProps...)
		}
		for key, val := range refLocations {
			locations[key] = val
		}
	}
	return locations, props
}

func differentPackage(root, ref *apiext.JSONSchemaProps) bool {
	return packageForSchema(root) != packageForSchema(ref)
}

func (g Generator) executeTemplate(ctx *genall.GenerationContext, name string, input interface{}, fileName string) error {
	var buffer bytes.Buffer
	if err := g.template.ExecuteTemplate(&buffer, name, input); err != nil {
		return err
	}
	out, err := ctx.Open(nil, fileName)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = buffer.WriteTo(out)
	return err
}

type flattener struct {
	Parser *crd.Parser
}

func (f *flattener) flattenSchema(baseSchema apiext.JSONSchemaProps, currentPackage *loader.Package) *apiext.JSONSchemaProps {
	resSchema := baseSchema.DeepCopy()
	definitions := make(apiext.JSONSchemaDefinitions)
	crd.EditSchema(resSchema, &flattenVisitor{
		flattener:      f,
		basePackage:    currentPackage,
		currentPackage: currentPackage,
		uniqueNames:    make(map[crd.TypeIdent]string),
		definitions:    definitions,
	})

	resSchema.Definitions = definitions

	return resSchema
}

type flattenVisitor struct {
	*flattener

	basePackage    *loader.Package
	currentPackage *loader.Package
	currentSchema  *apiext.JSONSchemaProps
	uniqueNames    map[crd.TypeIdent]string
	refName        string
	definitions    apiext.JSONSchemaDefinitions
}

func (f *flattenVisitor) Visit(baseSchema *apiext.JSONSchemaProps) crd.SchemaVisitor {
	if baseSchema == nil {
		if f.currentSchema != nil {
			refName := f.refName
			f.currentSchema.Ref = &refName
			f.refName = ""
			f.currentSchema = nil
		}
	} else if baseSchema.Ref != nil && len(*baseSchema.Ref) > 0 {
		// resolve this ref
		refIdent, err := f.identFromRef(*baseSchema.Ref, f.currentPackage)
		if err != nil {
			f.currentPackage.AddError(errors.Wrapf(err, "error identifying reference: name: %s, package: %s, schemaID: %s", refIdent.Name, refIdent.Package.Name, baseSchema.ID))
			return nil
		}

		uniqueName := f.getUniqueName(refIdent)
		f.currentSchema = baseSchema
		f.refName = "#/definitions/" + uniqueName

		// check to see if we've already processed this reference
		if _, ok := f.definitions[uniqueName]; ok {
			return f
		}

		// add it now
		refSchema, err := f.loadUnflattenedSchema(refIdent)
		if err != nil {
			keys := make([]string, len(f.definitions))
			index := 0
			for key := range f.definitions {
				keys[index] = key
			}
			f.currentPackage.AddError(errors.Wrapf(err, "error loading reference schema: name: %s, package: %s, definitions: %s", refIdent.Name, refIdent.Package.Name, keys))
			return nil
		}
		refSchema = refSchema.DeepCopy()
		refSchema.ID = f.getSchemaID(refIdent)
		f.definitions[uniqueName] = *refSchema

		// collect nested references
		nestedVistior := *f
		nestedVistior.currentPackage = refIdent.Package
		nestedVistior.currentSchema = nil
		nestedVistior.refName = ""
		crd.EditSchema(refSchema, &nestedVistior)
	}

	return f
}

func (f *flattenVisitor) getSchemaID(refIdent crd.TypeIdent) string {
	return fmt.Sprintf("urn:%s/%s", refIdent.Package.PkgPath, refIdent.Name)
}

func (f *flattenVisitor) getUniqueName(refIdent crd.TypeIdent) string {
	if name, ok := f.uniqueNames[refIdent]; ok {
		return name
	}
	name := refIdent.Name
	defer func() {
		f.uniqueNames[refIdent] = name
	}()
	if refIdent.Package == f.basePackage {
		return name
	}
	parts := strings.Split(refIdent.Package.PkgPath, "/")
	partsLen := len(parts)
	if partsLen > 0 {
		partsLen--
		name = fmt.Sprintf("%s_%s", parts[partsLen], name)
		partsLen--
	}
	// as most packages will be a version identifier, start with the last two segments
	for exists := true; exists && partsLen > 0; partsLen-- {
		name = fmt.Sprintf("%s%s", parts[partsLen], name)
		_, exists = f.definitions[name]
	}
	return name
}

// loadUnflattenedSchema fetches a fresh, unflattened schema from the parser.
// culled from sigs.k8s.io/controller-tools/pkg/crd/flatten.go
func (f *flattener) loadUnflattenedSchema(typ crd.TypeIdent) (*apiext.JSONSchemaProps, error) {
	f.Parser.NeedSchemaFor(typ)

	baseSchema, found := f.Parser.Schemata[typ]
	if !found {
		return nil, fmt.Errorf("unable to locate schema for type %s", typ)
	}
	return &baseSchema, nil
}

// identFromRef converts the given schema ref from the given package back
// into the TypeIdent that it represents.
// culled from sigs.k8s.io/controller-tools/pkg/crd/flatten.go
func (f *flattener) identFromRef(ref string, contextPkg *loader.Package) (crd.TypeIdent, error) {
	typ, pkgName, err := crd.RefParts(ref)
	if err != nil {
		return crd.TypeIdent{}, err
	}

	if pkgName == "" {
		// a local reference
		return crd.TypeIdent{
			Name:    typ,
			Package: contextPkg,
		}, nil
	}

	// an external reference
	return crd.TypeIdent{
		Name:    typ,
		Package: contextPkg.Imports()[pkgName],
	}, nil
}
