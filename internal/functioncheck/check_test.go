package functioncheck

import (
	"strings"
	"testing"
)

func TestPhysicalLineBoundary(t *testing.T) {
	for _, bodyLines := range []int{98, 99} {
		code := "package fixture\nfunc Sample() {\n" + strings.Repeat("// body\n", bodyLines) + "}\n"
		findings, err := CheckFile("fixture.go", []byte(code))
		if err != nil {
			t.Fatal(err)
		}
		if bodyLines == 98 && len(findings) != 0 {
			t.Fatal("100-line function rejected")
		}
		if bodyLines == 99 && (len(findings) != 1 || findings[0].Name != "Sample" || findings[0].Line != 2 || findings[0].Lines != 101) {
			t.Fatal("101-line declaration was not reported accurately")
		}
	}
}

func TestClosuresAndPlatformSpecificFilesAreChecked(t *testing.T) {
	code := "//go:build windows\n\npackage fixture\nvar F = func() {\n" + strings.Repeat("// body\n", 99) + "}\n"
	findings, err := CheckFile("fixture_windows.go", []byte(code))
	if err != nil || len(findings) != 1 || findings[0].Name != "<closure>" || findings[0].Lines != 101 {
		t.Fatalf("platform closure not checked: count=%d error=%v", len(findings), err)
	}
}

func TestSyntaxAndNonFunctionText(t *testing.T) {
	code := "package fixture\nvar S = `func Pretend() {\n" + strings.Repeat("text\n", 110) + "}`\nfunc Native()\nfunc Identity[T any](v T) T { return v }\n"
	if findings, err := CheckFile("fixture.go", []byte(code)); err != nil || len(findings) != 0 {
		t.Fatal("non-function text or a short generic declaration was misclassified")
	}
	if _, err := CheckFile("broken.go", []byte("package fixture\nfunc broken(")); err == nil {
		t.Fatal("syntax error silently accepted")
	}
	if findings, err := CheckFile("fixture_test.go", []byte("test fixture excluded")); err != nil || len(findings) != 0 {
		t.Fatal("test source was included in the production limit")
	}
}
