// gen generates Cobra command files from an OpenAPI spec.
// Usage: go run ./tools/gen --spec=<path> --out=<dir>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// ──────────────────────────────────────────────────────────────────────────────
// OpenAPI data structures (minimal subset)
// ──────────────────────────────────────────────────────────────────────────────

type Spec struct {
	Tags       []SpecTag           `yaml:"tags"`
	Paths      map[string]PathItem `yaml:"paths"`
	Components Components          `yaml:"components"`
}

type SpecTag struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type PathItem struct {
	Parameters []Parameter `yaml:"parameters"`
	Get        *Operation  `yaml:"get"`
	Post       *Operation  `yaml:"post"`
	Put        *Operation  `yaml:"put"`
	Delete     *Operation  `yaml:"delete"`
	Patch      *Operation  `yaml:"patch"`
}

type Operation struct {
	OperationID string       `yaml:"operationId"`
	Summary     string       `yaml:"summary"`
	Tags        []string     `yaml:"tags"`
	Parameters  []Parameter  `yaml:"parameters"`
	RequestBody *RequestBody `yaml:"requestBody"`
}

type Parameter struct {
	Name        string `yaml:"name"`
	In          string `yaml:"in"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
	Schema      Schema `yaml:"schema"`
}

type RequestBody struct {
	Content map[string]MediaType `yaml:"content"`
}

type MediaType struct {
	Schema Schema `yaml:"schema"`
}

type Schema struct {
	Ref                  string            `yaml:"$ref"`
	Type                 string            `yaml:"type"`
	Nullable             bool              `yaml:"nullable"`
	Properties           map[string]Schema `yaml:"properties"`
	Required             []string          `yaml:"required"`
	AdditionalProperties *Schema           `yaml:"additionalProperties"`
	AllOf                []Schema          `yaml:"allOf"`
	Format               string            `yaml:"format"`
	Description          string            `yaml:"description"`
	Default              any               `yaml:"default"`
	Enum                 []any             `yaml:"enum"`
}

type Components struct {
	Schemas map[string]Schema `yaml:"schemas"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Tag config – tags to generate files for
// ──────────────────────────────────────────────────────────────────────────────

type tagConfig struct {
	fileBase    string   // output file base name, e.g. "functions"
	varName     string   // Go var/command name, e.g. "functions"
	commandUse  string   // cobra Use string, e.g. "functions"
	stripSuffix []string // suffixes stripped from kebab-case operationId
}

var tagConfigs = map[string]tagConfig{
	"Functions":  {fileBase: "functions", varName: "functions", commandUse: "functions", stripSuffix: []string{"functions", "function"}},
	"Versions":   {fileBase: "versions", varName: "versions", commandUse: "versions", stripSuffix: []string{"versions", "version"}},
	"Executions": {fileBase: "executions", varName: "executions", commandUse: "executions", stripSuffix: []string{"executions", "execution"}},
	"API Tokens": {fileBase: "tokens", varName: "tokens", commandUse: "tokens", stripSuffix: []string{"tokens", "token"}},
}

// tagOrder controls the file generation order.
var tagOrder = []string{"Functions", "Versions", "Executions", "API Tokens"}

// commandNameOverrides maps operationId to an explicit subcommand name.
var commandNameOverrides = map[string]string{
	"updateEnvVars":             "env",
	"updateKV":                  "kv",
	"getNextRun":                "next-run",
	"getVersionDiff":            "diff",
	"getExecutionLogs":          "logs",
	"getExecutionAIRequests":    "ai-requests",
	"getExecutionEmailRequests": "email-requests",
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal operation model
// ──────────────────────────────────────────────────────────────────────────────

type fieldInfo struct {
	flagName     string // kebab-case flag name
	goVarSuffix  string // PascalCase suffix for var name
	goType       string // "string", "int", "bool", "[]string"
	defaultVal   string // Go literal default value
	desc         string
	required     bool
	isPointer    bool   // optional → pointer field in body struct
	isMap        bool   // map[string]string body field
	goFieldName  string // PascalCase name in the Go struct
	isCode       bool   // code field: support "-" for stdin
	enumCastType string // non-empty if oapi-codegen uses a custom enum type, e.g. "client.UpdateFunctionRequestCronStatus"
}

type pathArg struct {
	paramName   string // original param name, e.g. "id", "version", "versionId"
	goType      string // "string" or "int"
	displayName string // display in Use string, e.g. "<id>", "<version>"
}

type opInfo struct {
	operationID    string
	commandName    string
	summary        string
	goFuncName     string // PascalCase operationId, for use with WithResponse
	pathArgs       []pathArg
	queryFields    []fieldInfo
	bodyFields     []fieldInfo
	hasBody        bool
	bodyTypeName   string // e.g. "client.CreateFunctionJSONRequestBody"
	bodySchemaName string // resolved schema name, e.g. "UpdateFunctionRequest"
}

// ──────────────────────────────────────────────────────────────────────────────
// Main
// ──────────────────────────────────────────────────────────────────────────────

func main() {
	specPath := flag.String("spec", "", "path to openapi.yaml")
	outDir := flag.String("out", "", "output directory for generated .gen.go files")
	flag.Parse()

	if *specPath == "" || *outDir == "" {
		fmt.Fprintln(os.Stderr, "usage: gen --spec=<path> --out=<dir>")
		os.Exit(1)
	}

	data, err := os.ReadFile(*specPath)
	if err != nil {
		fatalf("reading spec: %v", err)
	}
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		fatalf("parsing spec: %v", err)
	}

	// Collect tag descriptions from the spec.
	tagDescs := make(map[string]string)
	for _, t := range spec.Tags {
		tagDescs[t.Name] = t.Description
	}

	// Group operations by tag, in path order (sorted for determinism).
	paths := sortedPaths(spec.Paths)
	tagOps := make(map[string][]opInfo)

	for _, path := range paths {
		item := spec.Paths[path]
		// Merge path-level parameters into each operation.
		for _, op := range []*Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch} {
			if op == nil || op.OperationID == "" {
				continue
			}
			merged := mergeParams(item.Parameters, op.Parameters)
			op.Parameters = merged

			// Only process tags we care about.
			for _, tag := range op.Tags {
				if _, ok := tagConfigs[tag]; !ok {
					continue
				}
				info, err := buildOpInfo(op, path, tag, &spec)
				if err != nil {
					fatalf("building op %s: %v", op.OperationID, err)
				}
				tagOps[tag] = append(tagOps[tag], info)
			}
		}
	}

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fatalf("creating output dir: %v", err)
	}

	for _, tag := range tagOrder {
		ops, ok := tagOps[tag]
		if !ok {
			continue
		}
		cfg := tagConfigs[tag]
		outFile := filepath.Join(*outDir, cfg.fileBase+".gen.go")
		src, err := generateFile(tag, tagDescs[tag], cfg, ops)
		if err != nil {
			fatalf("generating %s: %v", outFile, err)
		}
		if err := os.WriteFile(outFile, src, 0644); err != nil {
			fatalf("writing %s: %v", outFile, err)
		}
		fmt.Printf("wrote %s (%d operations)\n", outFile, len(ops))
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Operation model builder
// ──────────────────────────────────────────────────────────────────────────────

func buildOpInfo(op *Operation, path, tag string, spec *Spec) (opInfo, error) {
	cfg := tagConfigs[tag]
	info := opInfo{
		operationID: op.OperationID,
		commandName: deriveCommandName(op.OperationID, cfg),
		summary:     op.Summary,
		goFuncName:  toPascal(op.OperationID),
	}

	// Path and query params.
	for _, p := range op.Parameters {
		switch p.In {
		case "path":
			goType := schemaGoType(p.Schema)
			display := "<" + camelToKebab(p.Name) + ">"
			info.pathArgs = append(info.pathArgs, pathArg{
				paramName:   p.Name,
				goType:      goType,
				displayName: display,
			})
		case "query":
			f := buildFieldInfo(p.Name, "", p.Schema, p.Required, p.Description)
			info.queryFields = append(info.queryFields, f)
		}
	}

	// Request body.
	if op.RequestBody != nil {
		info.hasBody = true
		info.bodyTypeName = "client." + toPascal(op.OperationID) + "JSONRequestBody"

		mt, ok := op.RequestBody.Content["application/json"]
		if !ok {
			return info, nil
		}
		// Extract the schema name from the $ref before resolving.
		schemaName := strings.TrimPrefix(mt.Schema.Ref, "#/components/schemas/")

		schema := resolveRef(mt.Schema, spec)
		// Merge allOf schemas.
		schema = flattenAllOf(schema, spec)
		info.bodySchemaName = schemaName

		requiredSet := make(map[string]bool)
		for _, r := range schema.Required {
			requiredSet[r] = true
		}

		// Sort property names for deterministic output.
		propNames := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			propNames = append(propNames, name)
		}
		sort.Strings(propNames)

		for _, name := range propNames {
			prop := schema.Properties[name]
			required := requiredSet[name]
			f := buildFieldInfo(name, schemaName, prop, required, prop.Description)
			f.isPointer = !required
			info.bodyFields = append(info.bodyFields, f)
		}
	}

	return info, nil
}

// buildFieldInfo builds a fieldInfo for a single OpenAPI parameter/property.
// schemaName is the parent schema name (e.g. "UpdateFunctionRequest") used to derive
// the oapi-codegen enum type name; pass "" for query/path params.
func buildFieldInfo(name, schemaName string, schema Schema, required bool, desc string) fieldInfo {
	f := fieldInfo{
		flagName:    camelToKebab(snakeToKebab(name)),
		goVarSuffix: snakeToPascal(name),
		goFieldName: snakeToPascal(name),
		required:    required,
		desc:        escapeQuotes(desc),
	}

	// Detect map type.
	if schema.Type == "object" && schema.AdditionalProperties != nil {
		f.isMap = true
		f.goType = "[]string"
		f.defaultVal = "nil"
		// Use shorter flag name for well-known maps.
		if name == "env_vars" {
			f.flagName = "env"
		}
		return f
	}

	// Detect code field.
	if name == "code" && schema.Type == "string" {
		f.isCode = true
	}

	switch schema.Type {
	case "integer":
		f.goType = "int"
		if schema.Default != nil {
			f.defaultVal = fmt.Sprintf("%v", schema.Default)
		} else {
			f.defaultVal = "0"
		}
		// If the schema has enum values, oapi-codegen generates a custom type.
		if len(schema.Enum) > 0 && schemaName != "" {
			f.enumCastType = "client." + schemaName + f.goVarSuffix
		}
	case "boolean":
		f.goType = "bool"
		f.defaultVal = "false"
	default: // string, ""
		f.goType = "string"
		f.defaultVal = `""`
		// If the schema has enum values, oapi-codegen generates a custom string type.
		if len(schema.Enum) > 0 && schemaName != "" {
			f.enumCastType = "client." + schemaName + f.goVarSuffix
		}
	}
	return f
}

// ──────────────────────────────────────────────────────────────────────────────
// Code generator
// ──────────────────────────────────────────────────────────────────────────────

func generateFile(tag, tagDesc string, cfg tagConfig, ops []opInfo) ([]byte, error) {
	var b bytes.Buffer

	// Determine required imports.
	needsStrconv := false
	needsIO := false
	needsOS := false
	needsStrings := false
	needsClient := false
	for _, op := range ops {
		for _, a := range op.pathArgs {
			if a.goType == "int" {
				needsStrconv = true
			}
		}
		if len(op.queryFields) > 0 || op.hasBody {
			needsClient = true
		}
		for _, f := range op.bodyFields {
			if f.isCode {
				needsIO = true
				needsOS = true
			}
			if f.isMap {
				needsStrings = true
			}
		}
	}

	w(&b, "// Code generated by tools/gen; DO NOT EDIT.\n\n")
	w(&b, "package cmd\n\n")
	w(&b, "import (\n")
	w(&b, "\t\"fmt\"\n")
	if needsIO {
		w(&b, "\t\"io\"\n")
	}
	if needsOS {
		w(&b, "\t\"os\"\n")
	}
	if needsStrconv {
		w(&b, "\t\"strconv\"\n")
	}
	if needsStrings {
		w(&b, "\t\"strings\"\n")
	}
	w(&b, "\n")
	if needsClient {
		w(&b, "\t\"github.com/dimiro1/lunar/lunar-cli/client\"\n")
	}
	w(&b, "\t\"github.com/spf13/cobra\"\n")
	w(&b, ")\n\n")

	// Suppress unused import errors.
	w(&b, "var _ = fmt.Sprintf // suppress unused import\n\n")

	// Parent command.
	w(&b, "var %sCmd = &cobra.Command{\n", cfg.varName)
	w(&b, "\tUse:   %q,\n", cfg.commandUse)
	w(&b, "\tShort: %q,\n", tagDesc)
	w(&b, "}\n\n")

	w(&b, "func init() {\n")
	w(&b, "\trootCmd.AddCommand(%sCmd)\n", cfg.varName)
	w(&b, "}\n\n")

	// Each operation.
	for _, op := range ops {
		genOp(&b, cfg, op)
	}

	src, err := format.Source(b.Bytes())
	if err != nil {
		// Return unformatted for easier debugging.
		return b.Bytes(), fmt.Errorf("gofmt: %w (unformatted source written)", err)
	}
	return src, nil
}

func genOp(b *bytes.Buffer, cfg tagConfig, op opInfo) {
	prefix := op.operationID // e.g. "listFunctions"

	// Build Use string.
	var use strings.Builder
	use.WriteString(op.commandName)
	for _, a := range op.pathArgs {
		use.WriteString(" " + a.displayName)
	}

	// Command var.
	w(b, "// ─── %s ────────────────────────────────────────────────\n\n", op.commandName)
	w(b, "var %sCmd = &cobra.Command{\n", prefix)
	w(b, "\tUse:   %q,\n", use.String())
	w(b, "\tShort: %q,\n", op.summary)
	if len(op.pathArgs) > 0 {
		w(b, "\tArgs:  cobra.ExactArgs(%d),\n", len(op.pathArgs))
	}
	w(b, "\tRunE:  run%s,\n", op.goFuncName)
	w(b, "}\n\n")

	// Flag variable declarations.
	allFlags := append(op.queryFields, op.bodyFields...)
	if len(allFlags) > 0 {
		w(b, "var (\n")
		for _, f := range allFlags {
			varName := prefix + "Cmd" + f.goVarSuffix
			w(b, "\t%s %s\n", varName, f.goType)
		}
		w(b, ")\n\n")
	}

	// init(): register command + flags.
	w(b, "func init() {\n")
	w(b, "\t%sCmd.AddCommand(%sCmd)\n", cfg.varName, prefix)
	for _, f := range op.queryFields {
		varName := prefix + "Cmd" + f.goVarSuffix
		w(b, "\t%sCmd.Flags().%s(&%s, %q, %s, %q)\n",
			prefix, cobraFlagFunc(f.goType), varName, f.flagName, f.defaultVal, f.desc)
	}
	for _, f := range op.bodyFields {
		varName := prefix + "Cmd" + f.goVarSuffix
		if f.isMap {
			w(b, "\t%sCmd.Flags().StringArrayVar(&%s, %q, nil, %q)\n",
				prefix, varName, f.flagName, f.desc)
		} else {
			w(b, "\t%sCmd.Flags().%s(&%s, %q, %s, %q)\n",
				prefix, cobraFlagFunc(f.goType), varName, f.flagName, f.defaultVal, f.desc)
		}
		if f.required {
			w(b, "\t_ = %sCmd.MarkFlagRequired(%q)\n", prefix, f.flagName)
		}
	}
	w(b, "}\n\n")

	// RunE function.
	w(b, "func run%s(cmd *cobra.Command, args []string) error {\n", op.goFuncName)
	w(b, "\tc := mustClient()\n")

	// Decode integer path args.
	for i, a := range op.pathArgs {
		if a.goType == "int" {
			w(b, "\t%s, err := strconv.Atoi(args[%d])\n", a.paramName, i)
			w(b, "\tif err != nil {\n")
			w(b, "\t\treturn fmt.Errorf(\"invalid %s: %%w\", err)\n", a.paramName)
			w(b, "\t}\n")
		}
	}

	// Build query params struct if needed.
	if len(op.queryFields) > 0 {
		paramsType := "client." + op.goFuncName + "Params"
		w(b, "\tparams := &%s{\n", paramsType)
		for _, f := range op.queryFields {
			varName := prefix + "Cmd" + f.goVarSuffix
			if f.goType == "int" {
				w(b, "\t\t%s: &%s,\n", f.goFieldName, varName)
			} else {
				w(b, "\t\t%s: &%s,\n", f.goFieldName, varName)
			}
		}
		w(b, "\t}\n")
	}

	// Build request body if needed.
	if op.hasBody {
		genBodyConstruction(b, op, prefix)
	}

	// Build client call.
	w(b, "\tresp, err := c.%sWithResponse(cmd.Context()", op.goFuncName)

	// Path params.
	for i, a := range op.pathArgs {
		if a.goType == "int" {
			w(b, ", %s", a.paramName)
		} else {
			w(b, ", args[%d]", i)
		}
	}

	// Query params struct.
	if len(op.queryFields) > 0 {
		w(b, ", params")
	}

	// Body.
	if op.hasBody {
		w(b, ", body")
	}

	w(b, ")\n")
	w(b, "\tif err != nil {\n\t\treturn err\n\t}\n")
	w(b, "\treturn printAPIResponse(resp.StatusCode(), resp.Body)\n")
	w(b, "}\n\n")
}

func genBodyConstruction(b *bytes.Buffer, op opInfo, prefix string) {
	// Separate map fields from scalar fields.
	var mapFields, scalarFields []fieldInfo
	for _, f := range op.bodyFields {
		if f.isMap {
			mapFields = append(mapFields, f)
		} else {
			scalarFields = append(scalarFields, f)
		}
	}

	// Generate map parsing.
	for _, f := range mapFields {
		varName := prefix + "Cmd" + f.goVarSuffix
		mapVarName := f.flagName + "Map"
		w(b, "\t%s := make(map[string]string)\n", mapVarName)
		w(b, "\tfor _, kv := range %s {\n", varName)
		w(b, "\t\tparts := strings.SplitN(kv, \"=\", 2)\n")
		w(b, "\t\tif len(parts) != 2 {\n")
		w(b, "\t\t\treturn fmt.Errorf(\"invalid --%s format %%q, expected KEY=VALUE\", kv)\n", f.flagName)
		w(b, "\t\t}\n")
		w(b, "\t\t%s[parts[0]] = parts[1]\n", mapVarName)
		w(b, "\t}\n")
	}

	// Handle code-from-stdin for required code fields.
	for _, f := range scalarFields {
		if f.isCode && !f.isPointer {
			varName := prefix + "Cmd" + f.goVarSuffix
			w(b, "\tcode := %s\n", varName)
			w(b, "\tif code == \"-\" {\n")
			w(b, "\t\tb, err := io.ReadAll(os.Stdin)\n")
			w(b, "\t\tif err != nil {\n")
			w(b, "\t\t\treturn fmt.Errorf(\"reading stdin: %%w\", err)\n")
			w(b, "\t\t}\n")
			w(b, "\t\tcode = string(b)\n")
			w(b, "\t}\n")
		}
	}

	// Build body struct literal.
	w(b, "\tbody := %s{\n", op.bodyTypeName)
	for _, f := range scalarFields {
		varName := prefix + "Cmd" + f.goVarSuffix
		if f.isPointer {
			continue // set below via Changed()
		}
		if f.isCode {
			w(b, "\t\t%s: code,\n", f.goFieldName)
		} else {
			w(b, "\t\t%s: %s,\n", f.goFieldName, varName)
		}
	}
	for _, f := range mapFields {
		mapVarName := f.flagName + "Map"
		w(b, "\t\t%s: %s,\n", f.goFieldName, mapVarName)
	}
	w(b, "\t}\n")

	// Set optional (pointer) fields via Changed().
	for _, f := range scalarFields {
		if !f.isPointer {
			continue
		}
		varName := prefix + "Cmd" + f.goVarSuffix
		if f.isCode {
			// Support stdin for optional code fields too.
			w(b, "\tif cmd.Flags().Changed(%q) {\n", f.flagName)
			w(b, "\t\tcodeVal := %s\n", varName)
			w(b, "\t\tif codeVal == \"-\" {\n")
			w(b, "\t\t\tb, err := io.ReadAll(os.Stdin)\n")
			w(b, "\t\t\tif err != nil {\n")
			w(b, "\t\t\t\treturn fmt.Errorf(\"reading stdin: %%w\", err)\n")
			w(b, "\t\t\t}\n")
			w(b, "\t\t\tcodeVal = string(b)\n")
			w(b, "\t\t}\n")
			w(b, "\t\tbody.%s = &codeVal\n", f.goFieldName)
			w(b, "\t}\n")
		} else if f.enumCastType != "" {
			// oapi-codegen uses a custom type for enum fields; cast before taking address.
			w(b, "\tif cmd.Flags().Changed(%q) {\n", f.flagName)
			w(b, "\t\tv := %s(%s)\n", f.enumCastType, varName)
			w(b, "\t\tbody.%s = &v\n", f.goFieldName)
			w(b, "\t}\n")
		} else {
			w(b, "\tif cmd.Flags().Changed(%q) {\n", f.flagName)
			w(b, "\t\tv := %s\n", varName)
			w(b, "\t\tbody.%s = &v\n", f.goFieldName)
			w(b, "\t}\n")
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func w(b *bytes.Buffer, format string, args ...any) {
	fmt.Fprintf(b, format, args...)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen: "+format+"\n", args...)
	os.Exit(1)
}

func sortedPaths(paths map[string]PathItem) []string {
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mergeParams(pathLevel, opLevel []Parameter) []Parameter {
	// Op-level params override path-level params with same name+in.
	seen := make(map[string]bool)
	for _, p := range opLevel {
		seen[p.In+":"+p.Name] = true
	}
	result := append([]Parameter{}, opLevel...)
	for _, p := range pathLevel {
		if !seen[p.In+":"+p.Name] {
			result = append(result, p)
		}
	}
	return result
}

func resolveRef(s Schema, spec *Spec) Schema {
	if s.Ref == "" {
		return s
	}
	// Expect "#/components/schemas/<Name>"
	name := strings.TrimPrefix(s.Ref, "#/components/schemas/")
	if resolved, ok := spec.Components.Schemas[name]; ok {
		return resolved
	}
	return s
}

func flattenAllOf(s Schema, spec *Spec) Schema {
	if len(s.AllOf) == 0 {
		return s
	}
	merged := Schema{
		Properties: make(map[string]Schema),
	}
	requiredSet := make(map[string]bool)
	for _, sub := range s.AllOf {
		sub = resolveRef(sub, spec)
		sub = flattenAllOf(sub, spec)
		maps.Copy(merged.Properties, sub.Properties)
		for _, r := range sub.Required {
			requiredSet[r] = true
		}
	}
	// Also merge top-level properties (if allOf is combined with properties).
	maps.Copy(merged.Properties, s.Properties)
	for _, r := range s.Required {
		requiredSet[r] = true
	}
	for r := range requiredSet {
		merged.Required = append(merged.Required, r)
	}
	return merged
}

func schemaGoType(s Schema) string {
	switch s.Type {
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	default:
		return "string"
	}
}

func cobraFlagFunc(goType string) string {
	switch goType {
	case "int":
		return "IntVar"
	case "bool":
		return "BoolVar"
	default:
		return "StringVar"
	}
}

func deriveCommandName(operationID string, cfg tagConfig) string {
	if name, ok := commandNameOverrides[operationID]; ok {
		return name
	}
	kebab := camelToKebab(operationID)
	for _, suffix := range cfg.stripSuffix {
		if before, ok := strings.CutSuffix(kebab, "-"+suffix); ok {
			return before
		}
	}
	return kebab
}

// camelToKebab converts "listFunctions" → "list-functions".
func camelToKebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// snakeToKebab converts "env_vars" → "env-vars".
func snakeToKebab(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

// snakeToPascal converts "env_vars" → "EnvVars".
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		b.WriteRune(unicode.ToUpper(runes[0]))
		b.WriteString(string(runes[1:]))
	}
	return b.String()
}

// toPascal converts "listFunctions" → "ListFunctions".
func toPascal(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func escapeQuotes(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	// Remove newlines from descriptions.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}

// defaultForSchema returns the Go literal default value for a field.
func defaultForSchema(s Schema) string {
	if s.Default != nil {
		switch v := s.Default.(type) {
		case int:
			return strconv.Itoa(v)
		case float64:
			return strconv.Itoa(int(v))
		case bool:
			if v {
				return "true"
			}
			return "false"
		case string:
			return `"` + v + `"`
		}
	}
	switch s.Type {
	case "integer":
		return "0"
	case "boolean":
		return "false"
	default:
		return `""`
	}
}
