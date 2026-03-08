package migrations

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
)

func Lint() error {
	if err := validateMigrations(orderedMigrations); err != nil {
		return err
	}

	if err := lintAutoMigrateUsage(); err != nil {
		return err
	}

	return nil
}

func LintLines() ([]string, error) {
	err := Lint()
	if err != nil {
		return []string{
			"lint_status=failed",
			fmt.Sprintf("lint_error=%s", err.Error()),
		}, err
	}
	return []string{
		"lint_status=ok",
		fmt.Sprintf("migration_count=%d", len(orderedMigrations)),
	}, nil
}

func lintAutoMigrateUsage() error {
	content, err := os.ReadFile(migrationSourcePath)
	if err != nil {
		return fmt.Errorf("failed to read migration source for lint: %w", err)
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, migrationSourcePath, content, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("failed to parse migration source for lint: %w", err)
	}

	constStringValues := findStringConstants(file)
	migrationItems := findOrderedMigrationItems(file)

	var offenders []string
	for _, migration := range migrationItems {
		version := migrationVersion(migration, constStringValues)
		if version == initialSchemaVersion {
			continue
		}
		if migrationCallsAutoMigrate(migration) {
			offenders = append(offenders, version)
		}
	}

	offenders = slices.Compact(offenders)
	if len(offenders) == 0 {
		return nil
	}
	return errors.New("new migrations must not call AutoMigrate directly; use internal/migrations/ops helpers instead")
}

func migrationSourceByVersion(content []byte, version string) string {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, migrationSourcePath, content, parser.SkipObjectResolution)
	if err != nil {
		return ""
	}

	constStringValues := findStringConstants(file)
	migrationItems := findOrderedMigrationItems(file)
	for _, migration := range migrationItems {
		if migrationVersion(migration, constStringValues) != version {
			continue
		}
		start := fileSet.Position(migration.Pos()).Offset
		end := fileSet.Position(migration.End()).Offset
		if start < 0 || end < start || end > len(content) {
			return ""
		}
		return string(content[start:end])
	}
	return ""
}

func findStringConstants(file *ast.File) map[string]string {
	constants := map[string]string{}
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for idx, name := range valueSpec.Names {
				if idx >= len(valueSpec.Values) {
					continue
				}
				value, ok := valueSpec.Values[idx].(*ast.BasicLit)
				if !ok || value.Kind != token.STRING {
					continue
				}
				unquoted, err := strconv.Unquote(value.Value)
				if err != nil {
					continue
				}
				constants[name.Name] = unquoted
			}
		}
	}
	return constants
}

func findOrderedMigrationItems(file *ast.File) []*ast.CompositeLit {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for idx, name := range valueSpec.Names {
				if name.Name != "orderedMigrations" || idx >= len(valueSpec.Values) {
					continue
				}
				list, ok := valueSpec.Values[idx].(*ast.CompositeLit)
				if !ok {
					continue
				}
				items := make([]*ast.CompositeLit, 0, len(list.Elts))
				for _, elt := range list.Elts {
					migrationItem, ok := elt.(*ast.CompositeLit)
					if ok {
						items = append(items, migrationItem)
					}
				}
				return items
			}
		}
	}
	return nil
}

func migrationVersion(migration *ast.CompositeLit, constants map[string]string) string {
	for _, elt := range migration.Elts {
		field, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok || key.Name != "Version" {
			continue
		}
		switch value := field.Value.(type) {
		case *ast.BasicLit:
			if value.Kind != token.STRING {
				return ""
			}
			unquoted, err := strconv.Unquote(value.Value)
			if err != nil {
				return ""
			}
			return unquoted
		case *ast.Ident:
			if resolvedVersion, ok := constants[value.Name]; ok {
				return resolvedVersion
			}
			return value.Name
		default:
			return ""
		}
	}
	return ""
}

func migrationCallsAutoMigrate(migration *ast.CompositeLit) bool {
	for _, elt := range migration.Elts {
		field, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok || key.Name != "Up" {
			continue
		}
		upFunc, ok := field.Value.(*ast.FuncLit)
		if !ok {
			return false
		}
		callsAutoMigrate := false
		ast.Inspect(upFunc.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if selector.Sel.Name == "AutoMigrate" {
				callsAutoMigrate = true
				return false
			}
			return true
		})
		return callsAutoMigrate
	}
	return false
}
