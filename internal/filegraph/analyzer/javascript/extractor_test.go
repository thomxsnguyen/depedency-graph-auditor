package javascript

import (
	"reflect"
	"testing"
)

func TestExtractImports(t *testing.T) {
	source := []byte(`
import value from "./value"
import type { Model } from '../model'
import "./setup"
export { helper } from "./helper"
export * from '../shared'
const common = require("./common")
const lazy = import('./lazy')
import React from "react"
const external = require("package")
const computed = import(variable)
object.require("./not-a-require")
// import ignored from "./comment"
/* require("./block-comment") */
const text = "import fake from './string'"
const template = ` + "`require('./template')`" + `
`)

	got, err := ExtractImports(source)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./value", "../model", "./setup", "./helper", "../shared", "./common", "./lazy"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports: got %v, want %v", got, want)
	}
}

func TestExtractImportsRejectsUnterminatedSyntax(t *testing.T) {
	for _, source := range []string{
		`/* never closed`,
		`import "./never-closed`,
		"const value = `never closed",
	} {
		if _, err := ExtractImports([]byte(source)); err == nil {
			t.Fatalf("expected error for %q", source)
		}
	}
}
