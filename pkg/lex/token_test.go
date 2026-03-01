package lex

import (
	"testing"
	"unicode/utf8"
)

func TestSingleTokenRecognition(t *testing.T) {
	input := "=+if{"
	expectedTokens := []string{"=", "+", "{"}
	resultTokens := []*Token{}

	for _, runeValue := range input {
		if token, ok := isRuneAToken(runeValue); ok {
			resultTokens = append(resultTokens, token)
		}
	}

	if len(resultTokens) != len(expectedTokens) {
		t.Errorf("expected %d tokens, got %d", len(expectedTokens), len(resultTokens))
	}

	for i, token := range resultTokens {
		if token.string() != expectedTokens[i] {
			t.Errorf("expected token %s, got %s", expectedTokens[i], token.string())
		}
	}
}

func TestNewTree(t *testing.T) {
	newTokens := []*Token{
		NewTokenFromString("if"),
		NewTokenFromString("int"),
		NewTokenFromString("invoke"),
	}

	tree, err := NewTree(newTokens)
	if err != nil {
		t.Fatalf("failed to create tree: %v", err)
	}

	if len(tree.m['i'].Root.tok) == 0 {
		t.Errorf("expected 'i' branch to be initialized")
	}
}

func TestTreeDuplicateToken(t *testing.T) {
	tokens := []*Token{
		NewTokenFromString("int"),
		NewTokenFromString("int"),
	}

	_, err := NewTree(tokens)
	if err == nil {
		t.Errorf("expected error for duplicate tokens, got none")
	}
}

func TestNewTokenFromString(t *testing.T) {
	token := NewTokenFromString("int")
	if token.rLen != utf8.RuneCountInString("int") {
		t.Errorf("expected token length %d, got %d", utf8.RuneCountInString("int"), token.rLen)
	}
	if !token.Valid(false) {
		t.Errorf("expected token to be valid, got invalid")
	}
	if token.string() != "int" {
		t.Errorf("expected token string 'int', got %s", token.string())
	}
	if !token.Valid(true) {
		t.Errorf("expected token to be valid, got invalid")
	}

}

func TestBranchAddToken(t *testing.T) {
	rootToken := NewTokenFromString("int")
	branch := NewBranch(rootToken)

	newToken := NewTokenFromString("integer")
	if err := branch.AddToken(newToken); err != nil {
		t.Fatalf("failed to add token to branch: %v", err)
	}

	if len(branch.distanceToChildren) == 0 {
		t.Errorf("expected children to be added to branch, got none")
	}

	if distances, ok := branch.HasMatches('e'); !ok || distances != 4 {
		t.Errorf("expected to find a match with distance 3, got %d or no match found", distances)
	}
}

func TestInvalidToken(t *testing.T) {
	rootToken := NewTokenFromString("int")
	branch := NewBranch(rootToken)

	invalidToken := NewTokenFromString("in")
	err := branch.AddToken(invalidToken)
	if err == nil {
		t.Errorf("expected error when adding shorter token, got none")
	}
}
