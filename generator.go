package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"

	"github.com/stoewer/go-strcase"

	"github.com/rancher/go-rancher/client"
)

const (
	SOURCE_OUTPUT_DIR = "../src/main/java/io/rancher"
)

var (
	blackListTypes        map[string]bool
	blackListActions      map[string]bool
	blackListIncludeables map[string]bool
	noConversionTypes     map[string]bool
	underscoreRegexp      = regexp.MustCompile(`([a-z])([A-Z])`)
	schemaExists          map[string]bool
)

type metadata struct {
	importTypes       map[string]bool
	actionImportTypes map[string]bool
}

type fieldMetadata struct {
	FieldType     string
	FieldRequired bool
}

func (m *metadata) importActionClass(class string) {
	m.actionImportTypes[class] = true
}

func (m *metadata) importClass(class string) {
	m.importTypes[class] = true
}

func (m metadata) ListImports() []string {
	imports := make([]string, len(m.importTypes))
	i := 0
	for k := range m.importTypes {
		imports[i] = k
		i++
	}
	return imports
}

func (m metadata) ListActionImports() []string {
	imports := make([]string, len(m.actionImportTypes))
	i := 0
	for k := range m.actionImportTypes {
		imports[i] = k
		i++
	}
	return imports
}

func init() {
	blackListTypes = make(map[string]bool)
	blackListTypes["schema"] = true
	blackListTypes["resource"] = true
	blackListTypes["collection"] = true

	blackListActions = make(map[string]bool)
	blackListActions["create"] = true
	blackListActions["update"] = true

	blackListIncludeables = make(map[string]bool)
	blackListIncludeables["clusters"] = true

	noConversionTypes = make(map[string]bool)
	noConversionTypes["string"] = true

	schemaExists = make(map[string]bool)
}

func getTypeMap(schema client.Schema) (map[string]fieldMetadata, metadata) {
	meta := metadata{
		importTypes:       map[string]bool{},
		actionImportTypes: map[string]bool{},
	}

	fieldMetadataMap := map[string]fieldMetadata{}

	for name, field := range schema.ResourceFields {
		if name == "id" {
			continue
		}

		if name == "default" {
			fieldMetadataMap[ToFirstUpper("default_flag")] = fieldMetadata{"Boolean", field.Required}
			continue
		}

		if name == "expr" {
			fieldMetadataMap[ToFirstUpper("expr_flag")] = fieldMetadata{"Integer", field.Required}
			continue
		}

		if name == "for" {
			fieldMetadataMap[ToFirstUpper("for_flag")] = fieldMetadata{"String", field.Required}
			continue
		}

		fieldName := ToFirstUpper(name)

		if strings.HasPrefix(field.Type, "reference") || strings.HasPrefix(field.Type, "date") || strings.HasPrefix(field.Type, "enum") {
			fieldMetadataMap[fieldName] = fieldMetadata{"String", field.Required}
		} else if strings.EqualFold(field.Type, "hostname") || strings.EqualFold(field.Type, "dnsLabel") || strings.EqualFold(field.Type, "dnsLabelRestricted") || strings.EqualFold(field.Type, "password") || strings.EqualFold(field.Type, "base64") {
			fieldMetadataMap[fieldName] = fieldMetadata{"String", field.Required}
		} else if strings.HasPrefix(field.Type, "array[reference[") {
			fieldMetadataMap[fieldName] = fieldMetadata{"List<String>", field.Required}
			meta.importClass("java.util.List")
		} else if strings.HasPrefix(field.Type, "array") {
			meta.importClass("java.util.List")
			switch field.Type {
			case "array[reference]":
				fallthrough
			case "array[date]":
				fallthrough
			case "array[enum]":
				fallthrough
			case "array[string]":
				fieldMetadataMap[fieldName] = fieldMetadata{"List<String>", field.Required}
			case "array[int]":
				fieldMetadataMap[fieldName] = fieldMetadata{"List<Integer>", field.Required}
			case "array[float64]":
				fieldMetadataMap[fieldName] = fieldMetadata{"List<Float>", field.Required}
			case "array[array[float]]":
				fieldMetadataMap[fieldName] = fieldMetadata{"List<List<Float>>", field.Required}
			case "array[json]":
				fieldMetadataMap[fieldName] = fieldMetadata{"List<Map<String, Object>>", field.Required}
			default:
				fieldType := strings.TrimPrefix(field.Type, "array[")
				fieldType = strings.TrimSuffix(fieldType, "]")
				//class := strings.TrimSuffix(ToFirstUpper(fieldType), "s")
				class := ToFirstUpper(fieldType)
				fieldMetadataMap[fieldName] = fieldMetadata{"List<" + class + ">", field.Required}
			}
		} else if strings.HasPrefix(field.Type, "map") {
			meta.importClass("java.util.Map")
			fieldMetadataMap[fieldName] = fieldMetadata{"Map<String, Object>", field.Required}
		} else if strings.HasPrefix(field.Type, "json") {
			meta.importClass("java.util.Map")
			fieldMetadataMap[fieldName] = fieldMetadata{"Map<String, Object>", field.Required}
		} else if strings.HasPrefix(field.Type, "boolean") {
			fieldMetadataMap[fieldName] = fieldMetadata{"Boolean", field.Required}
		} else if strings.HasPrefix(field.Type, "extensionPoint") {
			fieldMetadataMap[fieldName] = fieldMetadata{"Object", field.Required}
		} else if strings.HasPrefix(field.Type, "float") {
			fieldMetadataMap[fieldName] = fieldMetadata{"Float", field.Required}
		} else if strings.HasPrefix(field.Type, "int") {
			fieldMetadataMap[fieldName] = fieldMetadata{"Integer", field.Required}
		} else {
			fieldMetadataMap[fieldName] = fieldMetadata{ToFirstUpper(field.Type), field.Required}
		}
	}

	return fieldMetadataMap, meta
}

func getResourceActions(packageName string, schema client.Schema, m metadata) map[string]client.Action {
	result := map[string]client.Action{}
	for name, action := range schema.ResourceActions {
		if _, ok := schemaExists[action.Output]; ok {
			if _, ok2 := blackListActions[name]; ok2 {
				continue
			}
			class := ToFirstUpper(schema.Id)
			if action.Input != "" && ToFirstUpper(action.Input) != class {
				if packageName == "" {
					m.importActionClass("io.rancher.type." + ToFirstUpper(action.Input))
				} else {
					m.importActionClass("io.rancher.type." + packageName + "." + ToFirstUpper(action.Input))
				}
			}
			if ToFirstUpper(action.Output) != class {
				if packageName == "" {
					m.importActionClass("io.rancher.type." + ToFirstUpper(action.Output))
				} else {
					m.importActionClass("io.rancher.type." + packageName + "." + ToFirstUpper(action.Output))
				}
			}
			result[name] = action
		}
	}
	return result
}

func generateType(packageName string, useLombok bool, schema client.Schema) error {
	templateName := "type.template"
	if useLombok {
		templateName = "type-lombok.template"
	}
	return generateTemplate(schema, path.Join(SOURCE_OUTPUT_DIR, "type", packageName, ToFirstUpper(schema.Id)+".java"), packageName, templateName)
}

func generateService(packageName string, schema client.Schema, schemas client.Schemas) error {
	serviceSourceOutputDir := path.Join(SOURCE_OUTPUT_DIR, "service", packageName)
	for _, includeableLink := range schema.IncludeableLinks {
		if _, ok := blackListIncludeables[includeableLink]; ok {
			continue
		}

		classNamePrefix := ToFirstUpper(schema.Id) + capitalizeClassSuffix(includeableLink)
		err := generateTemplateIncludeable(getIncludeableSchema(schemas, includeableLink), getPrefix(schema), classNamePrefix, path.Join(serviceSourceOutputDir, classNamePrefix+"Service.java"), packageName, "includeable.template")
		if err != nil {
			return err
		}
	}
	return generateTemplate(schema, path.Join(serviceSourceOutputDir, ToFirstUpper(schema.Id)+"Service.java"), packageName, "service.template")
}

func capitalizeClassSuffix(includeableLink string) string {
	var capitalizedClassSuffix string
	switch includeableLink {
	case "consumedservices":
		capitalizedClassSuffix = "ConsumedServices"
	case "consumedbyservices":
		capitalizedClassSuffix = "ConsumedByServices"
	default:
		capitalizedClassSuffix = ToFirstUpper(includeableLink)
	}
	return capitalizedClassSuffix
}

func getIncludeableSchema(schemas client.Schemas, includeableLink string) client.Schema {
	var result client.Schema
	var schemaId string

	switch includeableLink {
	case "networkContainer":
		schemaId = "network"
	case "consumedService":
		schemaId = "service"
	case "consumedservices":
		schemaId = "service"
	case "consumedbyservices":
		schemaId = "service"
	case "targetInstance":
		schemaId = "instance"
	case "targetInstanceLinks":
		schemaId = "instanceLink"
	case "hostLabels":
		schemaId = "label"
	case "instanceLabels":
		schemaId = "label"
	case "authenticatedAsAccount":
		schemaId = "account"
	case "reportedAccount":
		schemaId = "account"
	case "privateIpAddress":
		schemaId = "ipAddress"
	case "publicIpAddress":
		schemaId = "ipAddress"
	case "privatePorts":
		schemaId = "port"
	case "publicPorts":
		schemaId = "port"
	default:
		if includeableLink[len(includeableLink)-3:] == "ses" {
			schemaId = includeableLink[:len(includeableLink)-2]
		} else if includeableLink[len(includeableLink)-1:] == "s" {
			schemaId = includeableLink[:len(includeableLink)-1]
		} else {
			schemaId = includeableLink
		}
	}

	for _, schema := range schemas.Data {
		if schema.Id == schemaId {
			result = schema
			return result
		}
	}

	return result
}

func getPrefix(schema client.Schema) string {
	var prefix string
	schemaId := schema.Id
	if schemaId[len(schemaId)-1:] == "s" {
		prefix = schemaId + "es"
	} else {
		prefix = schemaId + "s"
	}
	return prefix
}

func generateTemplateIncludeable(schema client.Schema, prefix string, classNamePrefix string, outputPath string, packageName string, templateName string) error {
	err := setupDirectory(path.Dir(outputPath))
	if err != nil {
		return err
	}

	output, err := os.Create(outputPath)

	if err != nil {
		return err
	}

	defer output.Close()

	fieldMetadataMap, metadata := getTypeMap(schema)
	data := map[string]interface{}{
		"schema":          schema,
		"classNamePrefix": classNamePrefix,
		"class":           ToFirstUpper(schema.Id),
		"collection":      ToFirstUpper(schema.Id) + "Collection",
		"structFields":    fieldMetadataMap,
		"resourceActions": getResourceActions(packageName, schema, metadata),
		"type":            schema.Id,
		"meta":            metadata,
		"prefix":          prefix,
		"packageName":     packageName,
	}

	funcMap := template.FuncMap{
		"toUpperCamelCase": ToUpperCamelCase,
		"toLowerCamelCase": ToLowerCamelCase,
		"toFirstUpper":     ToFirstUpper,
		"toFirstLower":     ToFirstLower,
		"toUpper":          strings.ToUpper,
		"toLower":          strings.ToLower,
		"capitalize":       ToFirstUpper,
		"substrFlag":       substrFlag,
	}

	typeTemplate, err := template.New(templateName).Funcs(funcMap).ParseFiles(templateName)
	if err != nil {
		return err
	}

	return typeTemplate.Execute(output, data)
}

func generateTemplate(schema client.Schema, outputPath string, packageName string, templateName string) error {
	err := setupDirectory(path.Dir(outputPath))
	if err != nil {
		return err
	}

	output, err := os.Create(outputPath)

	if err != nil {
		return err
	}

	defer output.Close()

	fieldMetadataMap, metadata := getTypeMap(schema)
	data := map[string]interface{}{
		"schema":          schema,
		"class":           ToFirstUpper(schema.Id),
		"collection":      ToFirstUpper(schema.Id) + "Collection",
		"structFields":    fieldMetadataMap,
		"resourceActions": getResourceActions(packageName, schema, metadata),
		"type":            schema.Id,
		"meta":            metadata,
		"packageName":     packageName,
	}

	funcMap := template.FuncMap{
		"toUpperCamelCase": ToUpperCamelCase,
		"toLowerCamelCase": ToLowerCamelCase,
		"toFirstUpper":     ToFirstUpper,
		"toFirstLower":     ToFirstLower,
		"toUpper":          strings.ToUpper,
		"toLower":          strings.ToLower,
		"capitalize":       ToFirstUpper,
		"substrFlag":       substrFlag,
	}

	typeTemplate, err := template.New(templateName).Funcs(funcMap).ParseFiles(templateName)
	if err != nil {
		return err
	}

	return typeTemplate.Execute(output, data)
}

func ToLowerCamelCase(input string) string {
	return strcase.LowerCamelCase(input)
}

func ToUpperCamelCase(input string) string {
	return strcase.UpperCamelCase(input)
}

func ToFirstUpper(input string) string {
	return strings.ToUpper(input[:1]) + input[1:]
}

func ToFirstLower(input string) string {
	return strings.ToLower(input[:1]) + input[1:]
}

func substrFlag(input string) string {
	return ToFirstLower(input[:len(input)-5])
}

func setupDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}

	return nil
}

func generateFiles(schemasFilePath string, packageName string, useLombok bool) error {
	schemaBytes, err := ioutil.ReadFile(schemasFilePath)
	if err != nil {
		return err
	}

	var schemas client.Schemas

	err = json.Unmarshal(schemaBytes, &schemas)
	if err != nil {
		return err
	}

	for _, schema := range schemas.Data {
		if _, ok := blackListTypes[schema.Id]; ok {
			continue
		}

		schemaExists[schema.Id] = true
	}

	for _, schema := range schemas.Data {
		if _, ok := blackListTypes[schema.Id]; ok {
			continue
		}

		err = generateType(packageName, useLombok, schema)
		if err != nil {
			return err
		}

		err = generateService(packageName, schema, schemas)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	useLombok := true
	packageName := ""
	schemasFilePath := path.Join("schemas", packageName, "schemas.json")
	fmt.Println(schemasFilePath)

	err := generateFiles(schemasFilePath, packageName, useLombok)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
