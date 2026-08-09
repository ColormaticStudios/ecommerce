package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const expectedOperationCount = 204

var methods = []string{"get", "post", "put", "patch", "delete", "head", "options"}

var standardProblemResponses = map[string]string{
	"400": "BadRequestProblem",
	"401": "AuthenticationRequiredProblem",
	"403": "ForbiddenProblem",
	"404": "NotFoundProblem",
	"409": "ConflictProblem",
	"412": "PreconditionFailedProblem",
	"413": "PayloadTooLargeProblem",
	"422": "ValidationProblem",
	"429": "TooManyRequestsProblem",
	"500": "InternalServerErrorProblem",
	"502": "BadGatewayProblem",
	"503": "ServiceUnavailableProblem",
	"504": "GatewayTimeoutProblem",
}

type specification struct {
	Security   yaml.Node               `yaml:"security"`
	Paths      map[string]pathItem     `yaml:"paths"`
	Components specificationComponents `yaml:"components"`
}

type pathItem map[string]operation

type operation struct {
	OperationID string              `yaml:"operationId"`
	Security    yaml.Node           `yaml:"security"`
	Responses   map[string]response `yaml:"responses"`
}

type response struct {
	Ref     string               `yaml:"$ref"`
	Content map[string]mediaType `yaml:"content"`
}

type mediaType struct {
	Schema schema `yaml:"schema"`
}

type schema struct {
	Ref string `yaml:"$ref"`
}

type specificationComponents struct {
	Responses map[string]response `yaml:"responses"`
}

type generatorConfig struct {
	Generate struct {
		Client bool `yaml:"client"`
	} `yaml:"generate"`
}

func main() {
	root := "."
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/openapi-contract [repository-root]")
		os.Exit(2)
	}
	if len(os.Args) == 2 {
		root = os.Args[1]
	}
	if err := validateRepository(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Validated strict-cutover contract prerequisites for %d operations.\n", expectedOperationCount)
}

func validateRepository(root string) error {
	var failures []string

	specData, err := os.ReadFile(filepath.Join(root, "api", "openapi.yaml"))
	if err != nil {
		return fmt.Errorf("read OpenAPI specification: %w", err)
	}
	var spec specification
	if err := yaml.Unmarshal(specData, &spec); err != nil {
		return fmt.Errorf("parse OpenAPI specification: %w", err)
	}

	operationCount := 0
	operationIDs := make(map[string]string)
	for path, item := range spec.Paths {
		for _, method := range methods {
			op, ok := item[method]
			if !ok {
				continue
			}
			operationCount++
			label := strings.ToUpper(method) + " " + path
			if op.OperationID == "" {
				failures = append(failures, label+": missing operationId")
			} else if previous, exists := operationIDs[op.OperationID]; exists {
				failures = append(failures, label+": duplicate operationId "+op.OperationID+" also used by "+previous)
			} else {
				operationIDs[op.OperationID] = label
			}

			requiredStatuses := []string{"400", "500"}
			if securityRequired(op.Security, spec.Security) {
				requiredStatuses = append(requiredStatuses, "401", "403")
			}
			for _, status := range requiredStatuses {
				if _, ok := op.Responses[status]; !ok {
					failures = append(failures, label+": missing required "+status+" Problem response")
				}
			}

			for status, declared := range op.Responses {
				if len(status) != 3 || (status[0] != '4' && status[0] != '5') {
					continue
				}
				if declared.Ref == "" {
					failures = append(failures, label+": error response "+status+" must reference a reusable Problem response")
					continue
				}
				const prefix = "#/components/responses/"
				if !strings.HasPrefix(declared.Ref, prefix) {
					failures = append(failures, label+": error response "+status+" has unsupported reference "+declared.Ref)
					continue
				}
				name := strings.TrimPrefix(declared.Ref, prefix)
				if expected, standard := standardProblemResponses[status]; standard && name != expected {
					failures = append(failures, label+": error response "+status+" must reference "+expected+", found "+name)
				}
				component, ok := spec.Components.Responses[name]
				if !ok {
					failures = append(failures, label+": error response "+status+" references missing component "+name)
					continue
				}
				problem, ok := component.Content["application/problem+json"]
				if !ok || problem.Schema.Ref != "#/components/schemas/Problem" {
					failures = append(failures, label+": error response "+status+" component "+name+" must contain application/problem+json Problem")
				}
				if _, hasJSON := component.Content["application/json"]; hasJSON {
					failures = append(failures, label+": error response "+status+" component "+name+" must not declare application/json")
				}
			}
		}
	}
	if operationCount != expectedOperationCount {
		failures = append(failures, fmt.Sprintf("expected %d operations, found %d", expectedOperationCount, operationCount))
	}

	configData, err := os.ReadFile(filepath.Join(root, "api", "oapi-codegen.yaml"))
	if err != nil {
		return fmt.Errorf("read oapi-codegen config: %w", err)
	}
	var config generatorConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("parse oapi-codegen config: %w", err)
	}
	if !config.Generate.Client {
		failures = append(failures, "api/oapi-codegen.yaml: generated Go client is not enabled")
	}

	generated, err := os.ReadFile(filepath.Join(root, "internal", "apicontract", "openapi.gen.go"))
	if err != nil {
		failures = append(failures, "internal/apicontract/openapi.gen.go: generated Go client artifact is absent")
	} else {
		text := string(generated)
		if !strings.Contains(text, "type ClientInterface interface") || !strings.Contains(text, "func NewClient(") {
			failures = append(failures, "internal/apicontract/openapi.gen.go: generated Go client declarations are absent")
		}
	}

	if len(failures) == 0 {
		return nil
	}
	sort.Strings(failures)
	return errors.New("OpenAPI strict-cutover validation failed:\n - " + strings.Join(failures, "\n - "))
}

func securityRequired(operationSecurity, globalSecurity yaml.Node) bool {
	security := operationSecurity
	if security.Kind == 0 {
		security = globalSecurity
	}
	return security.Kind == yaml.SequenceNode && len(security.Content) > 0
}
