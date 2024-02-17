// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/tools/go/ast/astutil"
	"gopkg.in/yaml.v3"
)

var (
	protobufRelatedVarNameRegex          = regexp.MustCompile(`^[Ff]ile_.*_proto(_.*)?$`)
	removeProtobufTagRegex               = regexp.MustCompile(`.*(json:"[^"]*").*`)
	removeProtobufTagReplacement         = "`$1`"
	extractJSONFromProtobufRegex         = regexp.MustCompile(`protobuf:".*json=(\w+).*"`)
	extractJSONFromProtobufReplacement   = `json:"$1"`
	buildJSONFromProtobufNameRegex       = regexp.MustCompile(`protobuf:".*name=(\w+).*"`)
	buildJSONFromProtobufNameReplacement = `json:"$1"`
)

type Config struct {
	InputFiles            []*InputFile     `yaml:"inputFiles"`
	GlobalTransformations *Transformations `yaml:"globalTransformations"`
	OutputFile            string           `yaml:"outputFile"`
	HeaderFile            string           `yaml:"headerFile"`
	Package               string           `yaml:"package"`
}

type InputFile struct {
	Path            string           `yaml:"path"`
	Transformations *Transformations `yaml:"transformations"`
}

type Transformations struct {
	RemoveImports              []string          `yaml:"removeImports"`
	RenameImports              map[string]string `yaml:"renameImports"`
	AddImports                 map[string]string `yaml:"addImports"`
	RemoveVars                 []string          `yaml:"removeVars"`
	RemoveTypes                []string          `yaml:"removeTypes"`
	PreserveTypes              []string          `yaml:"preserveTypes"`
	RemoveFunctions            []string          `yaml:"removeFunctions"`
	RemoveFields               []string          `yaml:"removeFields"`
	RenameFields               map[string]string `yaml:"renameFields"`
	RenameTypes                map[string]string `yaml:"renameTypes"`
	ReplaceFunctionReturnTypes map[string]string `yaml:"replaceFunctionReturnTypes"`
	ReplaceFieldTypes          map[string]string `yaml:"replaceFieldTypes"`
	ReplaceTypes               map[string]string `yaml:"replaceTypes"`
	AddTags                    map[string]string `yaml:"addTags"`
}

var config *Config

type FileTransformer struct {
	FileSet         *token.FileSet
	InputFile       string
	Transformations *Transformations
	Package         string
}

func main() {
	if len(os.Args) < 1 {
		log("No transformation file specified")
		os.Exit(1)
	}

	config = loadConfig(os.Args[1])

	fset := token.NewFileSet()

	var transformedFiles []*ast.File
	for _, inputFile := range config.InputFiles {
		fileTransformer := FileTransformer{
			FileSet:         fset,
			InputFile:       inputFile.Path,
			Transformations: merge(inputFile.Transformations, config.GlobalTransformations),
			Package:         config.Package,
		}
		file, err := fileTransformer.processFile()
		if err != nil {
			panic(err)
		}
		transformedFiles = append(transformedFiles, file)
	}

	mergedFile := mergeFiles(fset, transformedFiles)

	output := getFileHeader(config.HeaderFile) + goFmt(fset, mergedFile)
	output = removeLeadingEmptyLinesFromStructs(output)

	// write to outputFile
	if err := os.WriteFile(config.OutputFile, []byte(output), 0o644); err != nil {
		panic(err)
	}
}

func merge(local, global *Transformations) *Transformations {
	if local == nil {
		local = &Transformations{}
	}
	if global == nil {
		global = &Transformations{}
	}
	return &Transformations{
		RemoveImports:              mergeStringArrays(local.RemoveImports, global.RemoveImports),
		RenameImports:              mergeStringMaps(local.RenameImports, global.RenameImports),
		AddImports:                 mergeStringMaps(local.AddImports, global.AddImports),
		RemoveVars:                 mergeStringArrays(local.RemoveVars, global.RemoveVars),
		RemoveTypes:                mergeStringArrays(local.RemoveTypes, global.RemoveTypes),
		PreserveTypes:              mergeStringArrays(local.PreserveTypes, global.PreserveTypes),
		RemoveFunctions:            mergeStringArrays(local.RemoveFunctions, global.RemoveFunctions),
		RemoveFields:               mergeStringArrays(local.RemoveFields, global.RemoveFields),
		RenameFields:               mergeStringMaps(local.RenameFields, global.RenameFields),
		RenameTypes:                mergeStringMaps(local.RenameTypes, global.RenameTypes),
		ReplaceFunctionReturnTypes: mergeStringMaps(local.ReplaceFunctionReturnTypes, global.ReplaceFunctionReturnTypes),
		ReplaceFieldTypes:          mergeStringMaps(local.ReplaceFieldTypes, global.ReplaceFieldTypes),
		ReplaceTypes:               mergeStringMaps(local.ReplaceTypes, global.ReplaceTypes),
		AddTags:                    mergeStringMaps(local.AddTags, global.AddTags),
	}
}

func mergeStringArrays(arrays ...[]string) []string {
	result := []string{}
	for _, a := range arrays {
		if a != nil {
			result = append(result, a...)
		}
	}
	return result
}

func mergeStringMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func goFmt(fset *token.FileSet, file *ast.File) string {
	var buf strings.Builder
	if err := format.Node(&buf, fset, file); err != nil {
		panic(err)
	}
	return buf.String()
}

func (t *FileTransformer) processFile() (*ast.File, error) {
	// Parse the Go source file
	file, err := parser.ParseFile(t.FileSet, t.InputFile, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("error parsing file: %v", err)
	}

	t.renamePackage(file)

	removeProtoimplEnforceVersion(file)
	convertEnums(file)

	file.Comments = nil

	// Traverse the AST and make modifications
	ast.Inspect(file, func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				var newSpecs []ast.Spec

				for _, spec := range decl.Specs {
					if importSpec, ok := spec.(*ast.ImportSpec); ok {
						if !t.shouldRemoveImport(importSpec) {
							newSpecs = append(newSpecs, spec)
						}
					}
				}
				for alias, path := range t.Transformations.AddImports {
					newSpecs = append(newSpecs, &ast.ImportSpec{
						Name: &ast.Ident{Name: alias},
						Path: &ast.BasicLit{Kind: token.STRING, Value: `"` + path + `"`},
					})
				}
				decl.Specs = newSpecs
			}
		case *ast.FuncDecl:
			if decl.Type.Results != nil {
				for _, r := range decl.Type.Results.List {
					if newType := t.getFunctionReturnTypeReplacement(decl); newType != nil {
						r.Type = newType
					}
					if newType := t.getTypeMapping(r.Type); newType != nil {
						r.Type = newType
					}
				}
			}
		case *ast.TypeSpec:
			typeSpec := decl
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				structName := typeSpec.Name.Name

				// Traverse fields of the struct, creating a new filtered list of fields.
				var filteredList []*ast.Field
				for _, field := range structType.Fields.List {
					fieldName := field.Names[0].Name
					if t.fieldShouldBeRemoved(structName, fieldName) {
						continue
					}
					if newName := t.getFieldRename(structName, fieldName); newName != "" {
						fieldName = newName
					}
					if newType := t.getFieldTypeReplacement(structName, fieldName); newType != nil {
						field.Type = newType
					}
					if newType := t.getTypeMapping(field.Type); newType != nil {
						field.Type = newType
					}

					if tag := t.getFieldTags(structName, fieldName); tag != "" {
						addTag(field, tag)
					}
					if toString(field.Type) == "*intstr.IntOrString" {
						addTag(field, "// +kubebuilder:validation:XIntOrString")
					}

					if field.Doc != nil {
						for _, comment := range field.Doc.List {
							if strings.HasPrefix(comment.Text, "// REQUIRED.") {
								addTag(field, "// +kubebuilder:validation:Required")
								removeOmitemptyFromJSONTag(field)
								// TODO: remove pointer?
							}
						}
					}

					transformFieldTag(field)
					filteredList = append(filteredList, field)
				}
				structType.Fields.List = filteredList

				if newName := t.getTypeRename(structName); newName != "" {
					typeSpec.Name.Name = newName
				}
			}
		}
		return true
	})

	processInterfaceFields(file)
	t.filterDeclarations(file)

	t.renameImports(file)
	fixNames(file)
	removeEmptyBlocks(file)

	return file, nil
}

func mergeFiles(fset *token.FileSet, files []*ast.File) *ast.File {
	mergedFile := &ast.File{
		Package:  files[0].Package,
		Name:     files[0].Name,
		Decls:    make([]ast.Decl, 0),
		Scope:    nil,
		Imports:  make([]*ast.ImportSpec, 0),
		Comments: nil,
	}

	for _, file := range files {
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
				for _, spec := range genDecl.Specs {
					importSpec := spec.(*ast.ImportSpec)
					astutil.AddNamedImport(fset, mergedFile, importSpec.Name.Name, removeQuotes(importSpec.Path.Value))
				}
				continue
			}
			mergedFile.Decls = append(mergedFile.Decls, decl)
		}
	}

	return mergedFile
}

func getFileHeader(headerFile string) string {
	copyright, err := os.ReadFile(headerFile)
	if err != nil {
		panic(err)
	}
	return string(copyright) + "// Code generated by hack/api_transformer/main.go. DO NOT EDIT.\n\n"
}

func fixNames(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			x.Name = toCamelCase(x.Name)
		}
		return true
	})
}

func addTag(node ast.Node, text string) {
	switch n := node.(type) {
	case *ast.Field:
		if n.Doc == nil {
			n.Doc = &ast.CommentGroup{}
		}
		n.Doc.List = append(n.Doc.List, &ast.Comment{Text: text})
	case *ast.GenDecl:
		if n.Doc == nil {
			n.Doc = &ast.CommentGroup{}
		}
		n.Doc.List = append(n.Doc.List, &ast.Comment{Text: text})
	case *ast.TypeSpec:
		if n.Doc == nil {
			n.Doc = &ast.CommentGroup{}
		}
		n.Doc.List = append(n.Doc.List, &ast.Comment{Text: text})
	default:
		panic(fmt.Sprintf("unsupported type %T", n))
	}
}

// processInterfaceFields finds structs that contain a field of type interface, removes this field and
// then finds the structs that implement that interface and adds all their fields to the original struct.
// This is necessary for structs like ExtensionProvider
func processInterfaceFields(file *ast.File) {
	interfaceImplementations := make(map[string][]*ast.StructType)

	typesToRemove := make(map[string]struct{})

	// Traverse functions to find structs that implement interfaces
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Recv != nil {
				// Get the name of the struct implementing the interface
				structName := removeAsterisk(getReceiverType(funcDecl))
				// Get the interface being implemented
				interfaceName := funcDecl.Name.Name
				if structName != "" && strings.HasPrefix(interfaceName, "is") {
					// Store the struct type in the map
					if structType, ok := file.Scope.Lookup(structName).Decl.(*ast.TypeSpec).Type.(*ast.StructType); ok {
						interfaceImplementations[interfaceName] = append(interfaceImplementations[interfaceName], structType)
						typesToRemove[structName] = struct{}{}
						typesToRemove[interfaceName] = struct{}{}
					}
				}
			}
		}
	}

	// Traverse declarations to find struct with interface field and replace with implemented fields
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						// Check if the struct contains a field of interface type
						var newFields []*ast.Field
						hasInterfaceField := false
						var interfaceFields []*ast.Field
						for _, field := range structType.Fields.List {
							interfaceName := toString(field.Type)
							if implementations, found := interfaceImplementations[interfaceName]; found {
								// Add fields from implemented structs
								hasInterfaceField = true
								for _, implStruct := range implementations {
									for _, implField := range implStruct.Fields.List {
										interfaceFields = append(interfaceFields, implField)
										addOmitemptyToJSONTag(implField)
										newFields = append(newFields, implField)
									}
								}
								continue // skip adding the interface field
							}
							newFields = append(newFields, field)
						}
						if hasInterfaceField {
							addTag(genDecl, buildOneOfValidation(interfaceFields))
						}
						structType.Fields.List = newFields
					}
				}
			}
		}
	}

	// Remove interface and implementing structs
	astutil.Apply(file, func(cursor *astutil.Cursor) bool {
		if typeSpec, ok := cursor.Node().(*ast.TypeSpec); ok {
			if _, shouldRemove := typesToRemove[typeSpec.Name.Name]; shouldRemove {
				cursor.Delete()
				return false
			}
		}
		return true
	}, nil)
}

func removeAsterisk(name string) string {
	if len(name) > 1 && name[0] == '*' {
		return name[1:]
	}
	return name
}

func buildOneOfValidation(fields []*ast.Field) string {
	jsonNames := make([]string, len(fields))
	expressions := make([]string, len(fields))

	for i, f := range fields {
		jsonName := getJSONName(f)
		jsonNames[i] = jsonName
		expressions[i] = fmt.Sprintf("(has(self.%s)?1:0)", jsonName)
	}

	return fmt.Sprintf(`// +kubebuilder:validation:XValidation:message="At most one of %v should be set",rule="%s <= 1"`,
		jsonNames,
		strings.Join(expressions, " + "))
}

func getJSONName(field *ast.Field) string {
	regex := regexp.MustCompile(`json:"([^",]*).*"`)
	submatches := regex.FindStringSubmatch(field.Tag.Value)
	if len(submatches) > 0 {
		return submatches[1]
	}
	return ""
}

func addOmitemptyToJSONTag(field *ast.Field) {
	tag := field.Tag.Value
	field.Tag.Value = regexp.MustCompile(`json:"([^"]*)"`).ReplaceAllString(tag, `json:"$1,omitempty"`)
}

func removeOmitemptyFromJSONTag(field *ast.Field) {
	tag := field.Tag.Value
	field.Tag.Value = regexp.MustCompile(`json:"([^"]*),omitempty"`).ReplaceAllString(tag, `json:"$1"`)
}

func loadConfig(configFileName string) *Config {
	// Read YAML file
	yamlFile, err := os.ReadFile(configFileName)
	if err != nil {
		log("Error reading config file:", err)
		os.Exit(1)
	}

	// Unmarshal YAML into config struct
	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		logf("Error reading config file %s: %v", configFileName, err)
		os.Exit(1)
	}
	return &config
}

func removeEmptyBlocks(file *ast.File) {
	astutil.Apply(file, func(cursor *astutil.Cursor) bool {
		if genDecl, ok := cursor.Node().(*ast.GenDecl); ok {
			if len(genDecl.Specs) == 0 {
				cursor.Delete()
				return false
			}
		}
		return true
	}, nil)
}

func convertEnums(file *ast.File) {
	for _, e := range findEnums(file) {
		convertEnum(e, file)
	}
}

// findEnums finds all the enum types in the file by looking for int32 variables declared at the top level
// (this works for now, but may need to be improved in the future)
func findEnums(file *ast.File) []string {
	var enums []string
	ast.Inspect(file, func(node ast.Node) bool {
		if typeSpec, ok := node.(*ast.TypeSpec); ok {
			if toString(typeSpec.Type) == "int32" {
				enums = append(enums, typeSpec.Name.Name)
			}
		}
		return true
	})
	return enums
}

func convertEnum(enumName string, file *ast.File) {
	intsToEnumValues := make(map[string]string)
	var enumValues []string

	// Find the constant names and remove the variables `enumName + "_name"` and `enumName + "_value"`
	astutil.Apply(file, func(cursor *astutil.Cursor) bool {
		if valueSpec, ok := cursor.Node().(*ast.ValueSpec); ok {
			for _, n := range valueSpec.Names {
				if n.Name == enumName+"_name" {
					// read the map's entries and store them in intsToEnumValues
					if compositeLit, ok := (valueSpec.Values[0]).(*ast.CompositeLit); ok {
						for _, item := range compositeLit.Elts {
							if keyValueExpr, ok := item.(*ast.KeyValueExpr); ok {
								k := toString(keyValueExpr.Key)
								v := toString(keyValueExpr.Value)
								intsToEnumValues[k] = v
								enumValues = append(enumValues, removeQuotes(v))
							}
						}
					}
					cursor.Delete()
					return false
				} else if n.Name == enumName+"_value" {
					cursor.Delete()
					return false
				}
			}
		}
		return true
	}, nil)

	// update the enum type and constants
	ast.Inspect(file, func(node ast.Node) bool {
		if genDecl, ok := node.(*ast.GenDecl); ok {
			switch genDecl.Tok {
			case token.TYPE:
				// change the type to "string"
				typeSpec := genDecl.Specs[0].(*ast.TypeSpec)
				if typeSpec.Name.Name == enumName {
					if ident, ok := typeSpec.Type.(*ast.Ident); ok && ident.Name == "int32" {
						ident.Name = "string"
					}
					addTag(genDecl, "// +kubebuilder:validation:Enum="+strings.Join(enumValues, ";"))
				}
			case token.CONST:
				// change the constant values to strings
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						valueType := valueSpec.Type
						var constValue string
						if valueType != nil {
							if valueTypeIdent, ok := valueType.(*ast.Ident); ok && valueTypeIdent.Name == enumName {
								for i, value := range valueSpec.Values {
									if basicLit, ok := value.(*ast.BasicLit); ok {
										constValue = intsToEnumValues[basicLit.Value]
										valueSpec.Values[i] = &ast.BasicLit{
											Kind:  token.STRING,
											Value: constValue,
										}
									}
								}
								valueSpec.Names[0].Name = valueTypeIdent.Name + underscoreToCamelCase(removeQuotes(constValue))
							}
						}
					}
				}
			default: // do nothing
			}
		}
		return true
	})
}

func removeQuotes(s string) string {
	return s[1 : len(s)-1]
}

// underscoreToCamelCase converts a string from underscore-case to camel-case
func underscoreToCamelCase(input string) string {
	words := strings.Split(input, "_")
	caser := cases.Title(language.English)
	for i := 0; i < len(words); i++ {
		words[i] = caser.String(strings.ToLower(words[i]))
	}
	return strings.Join(words, "")
}

func (t *FileTransformer) shouldRemoveImport(importSpec *ast.ImportSpec) bool {
	for _, path := range t.Transformations.RemoveImports {
		if removeQuotes(importSpec.Path.Value) == path {
			return true
		}
	}
	return false
}

func removeLeadingEmptyLinesFromStructs(str string) string {
	var sb strings.Builder

	lines := strings.Split(str, "\n")
	var prevTrimmedLine string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		skipLine := trimmedLine == "" && strings.HasSuffix(prevTrimmedLine, "struct {")
		if !skipLine {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		prevTrimmedLine = trimmedLine
	}
	return sb.String()
}

func (t *FileTransformer) functionShouldBeRemoved(receiverType string, funcName string) bool {
	return matches(receiverType, funcName, t.Transformations.RemoveFunctions)
}

func (t *FileTransformer) fieldShouldBeRemoved(structName string, fieldName string) bool {
	return matches(structName, fieldName, t.Transformations.RemoveFields)
}

func (t *FileTransformer) getFunctionReturnTypeReplacement(funcDecl *ast.FuncDecl) ast.Expr {
	return parseExpr(getMapValue(getReceiverType(funcDecl), funcDecl.Name.Name, t.Transformations.ReplaceFunctionReturnTypes))
}

func (t *FileTransformer) getFieldTypeReplacement(structName string, fieldName string) ast.Expr {
	return parseExpr(getMapValue(structName, fieldName, t.Transformations.ReplaceFieldTypes))
}

func (t *FileTransformer) getFieldRename(structName string, fieldName string) string {
	return getMapValue(structName, fieldName, t.Transformations.RenameFields)
}

func (t *FileTransformer) getFieldTags(structName string, fieldName string) string {
	return getMapValue(structName, fieldName, t.Transformations.AddTags)
}

func getMapValue(parent string, child string, m map[string]string) string {
	if v, found := m[parent+"."+child]; found {
		return v
	}
	if v, found := m["*."+child]; found {
		return v
	}
	return ""
}

func matches(parent string, child string, list []string) bool {
	for _, item := range list {
		tokens := strings.Split(item, ".")
		if (tokens[0] == "*" || tokens[0] == parent) &&
			(tokens[1] == "*" || tokens[1] == child) {
			return true
		}
	}
	return false
}

func (t *FileTransformer) getTypeRename(typeName string) string {
	return t.Transformations.RenameTypes[typeName]
}

func (t *FileTransformer) getTypeMapping(fieldType ast.Expr) ast.Expr {
	if expr, found := t.Transformations.ReplaceTypes[toString(fieldType)]; found {
		return parseExpr(expr)
	}
	return nil
}

func toString(astExpr any) string {
	if astExpr == nil {
		return ""
	}
	var buf strings.Builder
	fset := token.NewFileSet()
	if err := format.Node(&buf, fset, astExpr); err != nil {
		panic(err)
	}
	return buf.String()
}

func parseExpr(expr string) ast.Expr {
	if expr == "" {
		return nil
	}
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		panic(err)
	}
	return parsed
}

func transformFieldTag(field *ast.Field) {
	tag := field.Tag
	if tag != nil {
		tag.Value = fixJSONFieldName(removeProtobufTagRegex.ReplaceAllString(tag.Value, removeProtobufTagReplacement))
		if strings.HasPrefix(tag.Value, "`protobuf:") {
			tag.Value = extractJSONFromProtobufRegex.ReplaceAllString(tag.Value, extractJSONFromProtobufReplacement)
		}
		if strings.HasPrefix(tag.Value, "`protobuf:") {
			tag.Value = buildJSONFromProtobufNameRegex.ReplaceAllString(tag.Value, buildJSONFromProtobufNameReplacement)
		}
	}
}

func fixJSONFieldName(s string) string {
	// values_types.pb.go has the following exceptions:
	if strings.HasPrefix(s, "`json:\"proxy_init") ||
		strings.HasPrefix(s, "`json:\"istio_cni") ||
		strings.HasPrefix(s, "`json:\"psp_cluster_role") ||
		strings.HasPrefix(s, "`json:\"resource_quotas") {
		return s
	}
	return toCamelCase(s)
}

func toCamelCase(s string) string {
	re := regexp.MustCompile(`[-_]\w`)
	res := re.ReplaceAllStringFunc(s, func(m string) string {
		return strings.ToUpper(m[1:])
	})
	return res
}

func (t *FileTransformer) filterDeclarations(file *ast.File) {
	// Inspect the AST and create a new list of declarations excluding functions
	astutil.Apply(file, func(cursor *astutil.Cursor) bool {
		switch decl := cursor.Node().(type) {
		case *ast.FuncDecl:
			receiverType := getReceiverType(decl)
			// remove functions listed in transformations.removeFunctions
			if t.functionShouldBeRemoved(receiverType, decl.Name.Name) {
				cursor.Delete()
				return false
			}
			// remove functions where the receiver is a type we are removing
			if t.shouldRemoveType(removeAsterisk(receiverType)) {
				cursor.Delete()
				return false
			}
		case *ast.GenDecl:
			if decl.Tok == token.VAR {
				for _, spec := range decl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						varName := valueSpec.Names[0].Name
						if t.shouldRemoveVar(varName) {
							cursor.Delete()
							return false
						}
					}
				}
			} else if decl.Tok == token.TYPE {
				for _, spec := range decl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if t.shouldRemoveType(typeSpec.Name.Name) {
							cursor.Delete()
							return false
						}
					}
				}
			} else if decl.Tok == token.CONST {
				for _, spec := range decl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						if t.shouldRemoveType(toString(valueSpec.Type)) {
							cursor.Delete()
							return false
						}
					}
				}
			}
		}
		return true
	}, nil)
}

func (t *FileTransformer) shouldRemoveVar(varName string) bool {
	if protobufRelatedVarNameRegex.MatchString(varName) {
		return true
	}
	for _, name := range t.Transformations.RemoveVars {
		if varName == name {
			return true
		}
	}
	return false
}

func (t *FileTransformer) shouldRemoveType(name string) bool {
	shouldRemove := false
	for _, v := range t.Transformations.RemoveTypes {
		if v == "*" || v == name {
			shouldRemove = true
			break
		}
	}
	for _, v := range t.Transformations.PreserveTypes {
		if v == name {
			return false
		}
	}
	return shouldRemove
}

func (t *FileTransformer) renameImports(file *ast.File) {
	astutil.Apply(file, func(cursor *astutil.Cursor) bool {
		switch node := cursor.Node().(type) {
		case *ast.SelectorExpr:
			if node.Sel != nil && node.X != nil {
				if ident, ok := node.X.(*ast.Ident); ok {
					if newName, found := t.Transformations.RenameImports[ident.Name]; found {
						ident.Name = newName
					}
				}
			}
		case *ast.ImportSpec:
			if node.Name != nil {
				if newName, found := t.Transformations.RenameImports[node.Name.Name]; found {
					node.Name.Name = newName
				}
			}
		}
		return true
	}, nil)
}

func (t *FileTransformer) renamePackage(file *ast.File) {
	if t.Package != "" {
		file.Name.Name = t.Package
	}
}

func getReceiverType(funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return ""
	}
	return toString(funcDecl.Recv.List[0].Type)
}

// removeProtoimplEnforceVersion removes the following constants:
//
//	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
//	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
func removeProtoimplEnforceVersion(file *ast.File) {
	astutil.Apply(file, func(cursor *astutil.Cursor) bool {
		if valueSpec, ok := cursor.Node().(*ast.ValueSpec); ok {
			// Check if it has only one value and the expression is a call to protoimpl.EnforceVersion()
			if len(valueSpec.Names) == 1 && len(valueSpec.Values) == 1 {
				if callExpr, ok := valueSpec.Values[0].(*ast.CallExpr); ok {
					if toString(callExpr.Fun) == "protoimpl.EnforceVersion" {
						cursor.Delete()
						return false
					}
				}
			}
		}
		return true
	}, nil)
}

func log(a ...any) {
	_, err := fmt.Fprintln(os.Stderr, a...)
	if err != nil {
		panic(err)
	}
}

func logf(format string, a ...any) {
	_, err := fmt.Fprintf(os.Stderr, format, a...)
	if err != nil {
		panic(err)
	}
}
