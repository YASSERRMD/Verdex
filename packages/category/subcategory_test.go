package category

import "testing"

func buildSubCategoryTaxonomy(t *testing.T) Taxonomy {
	t.Helper()
	tax := NewDefaultTaxonomy("IN")
	if err := tax.AddCategory("IN", Category{Code: "civil-contract", Name: "Civil - Contract", ParentCode: CodeCivil}); err != nil {
		t.Fatalf("AddCategory(civil-contract) error = %v", err)
	}
	if err := tax.AddCategory("IN", Category{Code: "civil-property", Name: "Civil - Property", ParentCode: CodeCivil}); err != nil {
		t.Fatalf("AddCategory(civil-property) error = %v", err)
	}
	if err := tax.AddCategory("IN", Category{Code: "civil-contract-lease", Name: "Civil - Contract - Lease", ParentCode: "civil-contract"}); err != nil {
		t.Fatalf("AddCategory(civil-contract-lease) error = %v", err)
	}
	return tax
}

func TestSubCategories(t *testing.T) {
	tax := buildSubCategoryTaxonomy(t)
	civil, _ := tax.Lookup("IN", CodeCivil)

	children := SubCategories(tax, "IN", civil)
	if len(children) != 2 {
		t.Fatalf("got %d children, want 2", len(children))
	}
	codes := map[CategoryCode]bool{}
	for _, c := range children {
		codes[c.Code] = true
	}
	if !codes["civil-contract"] || !codes["civil-property"] {
		t.Errorf("got children %v, want civil-contract and civil-property", codes)
	}

	t.Run("no children", func(t *testing.T) {
		labor, _ := tax.Lookup("IN", CodeLabor)
		got := SubCategories(tax, "IN", labor)
		if len(got) != 0 {
			t.Errorf("got %d children, want 0", len(got))
		}
	})

	t.Run("unknown jurisdiction", func(t *testing.T) {
		got := SubCategories(tax, "ZZ", civil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestParentChain(t *testing.T) {
	tax := buildSubCategoryTaxonomy(t)

	t.Run("top-level category chain is itself only", func(t *testing.T) {
		civil, _ := tax.Lookup("IN", CodeCivil)
		chain, err := ParentChain(tax, "IN", civil)
		if err != nil {
			t.Fatalf("ParentChain() error = %v, want nil", err)
		}
		if len(chain) != 1 || chain[0].Code != CodeCivil {
			t.Errorf("got chain %v, want [civil]", chain)
		}
	})

	t.Run("single-level sub-category resolves to parent", func(t *testing.T) {
		sub, _ := tax.Lookup("IN", "civil-contract")
		chain, err := ParentChain(tax, "IN", sub)
		if err != nil {
			t.Fatalf("ParentChain() error = %v, want nil", err)
		}
		wantCodes := []CategoryCode{"civil-contract", CodeCivil}
		if len(chain) != len(wantCodes) {
			t.Fatalf("got chain length %d, want %d", len(chain), len(wantCodes))
		}
		for i, c := range chain {
			if c.Code != wantCodes[i] {
				t.Errorf("chain[%d] = %q, want %q", i, c.Code, wantCodes[i])
			}
		}
	})

	t.Run("multi-level sub-category resolves full chain", func(t *testing.T) {
		leaf, _ := tax.Lookup("IN", "civil-contract-lease")
		chain, err := ParentChain(tax, "IN", leaf)
		if err != nil {
			t.Fatalf("ParentChain() error = %v, want nil", err)
		}
		wantCodes := []CategoryCode{"civil-contract-lease", "civil-contract", CodeCivil}
		if len(chain) != len(wantCodes) {
			t.Fatalf("got chain length %d, want %d: %v", len(chain), len(wantCodes), chain)
		}
		for i, c := range chain {
			if c.Code != wantCodes[i] {
				t.Errorf("chain[%d] = %q, want %q", i, c.Code, wantCodes[i])
			}
		}
	})

	t.Run("unresolvable parent errors", func(t *testing.T) {
		orphan := Category{Code: "orphan", Name: "Orphan", ParentCode: "does-not-exist"}
		_, err := ParentChain(tax, "IN", orphan)
		if err != ErrUnknownParent {
			t.Errorf("ParentChain() error = %v, want %v", err, ErrUnknownParent)
		}
	})

	t.Run("unknown jurisdiction errors", func(t *testing.T) {
		civil, _ := tax.Lookup("IN", CodeCivil)
		_, err := ParentChain(tax, "ZZ", civil)
		if err != ErrUnknownJurisdiction {
			t.Errorf("ParentChain() error = %v, want %v", err, ErrUnknownJurisdiction)
		}
	})

	t.Run("cyclic chain errors instead of looping forever", func(t *testing.T) {
		cyclic := make(Taxonomy)
		cyclic["IN"] = map[CategoryCode]Category{
			"a": {Code: "a", ParentCode: "b"},
			"b": {Code: "b", ParentCode: "a"},
		}
		_, err := ParentChain(cyclic, "IN", cyclic["IN"]["a"])
		if err != ErrUnknownParent {
			t.Errorf("ParentChain() error = %v, want %v", err, ErrUnknownParent)
		}
	})
}

func TestResolveParent(t *testing.T) {
	tax := buildSubCategoryTaxonomy(t)

	t.Run("top-level category has no parent", func(t *testing.T) {
		civil, _ := tax.Lookup("IN", CodeCivil)
		_, ok, err := ResolveParent(tax, "IN", civil)
		if err != nil {
			t.Fatalf("ResolveParent() error = %v, want nil", err)
		}
		if ok {
			t.Error("ok = true, want false for top-level category")
		}
	})

	t.Run("sub-category resolves its parent", func(t *testing.T) {
		sub, _ := tax.Lookup("IN", "civil-contract")
		parent, ok, err := ResolveParent(tax, "IN", sub)
		if err != nil {
			t.Fatalf("ResolveParent() error = %v, want nil", err)
		}
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if parent.Code != CodeCivil {
			t.Errorf("got parent %q, want %q", parent.Code, CodeCivil)
		}
	})

	t.Run("unresolvable parent errors", func(t *testing.T) {
		orphan := Category{Code: "orphan", ParentCode: "does-not-exist"}
		_, _, err := ResolveParent(tax, "IN", orphan)
		if err != ErrUnknownParent {
			t.Errorf("ResolveParent() error = %v, want %v", err, ErrUnknownParent)
		}
	})

	t.Run("unknown jurisdiction errors", func(t *testing.T) {
		sub, _ := tax.Lookup("IN", "civil-contract")
		_, _, err := ResolveParent(tax, "ZZ", sub)
		if err != ErrUnknownJurisdiction {
			t.Errorf("ResolveParent() error = %v, want %v", err, ErrUnknownJurisdiction)
		}
	})
}

func TestValidateSubCategory(t *testing.T) {
	tax := buildSubCategoryTaxonomy(t)

	t.Run("valid chain", func(t *testing.T) {
		leaf, _ := tax.Lookup("IN", "civil-contract-lease")
		if err := ValidateSubCategory(tax, "IN", leaf); err != nil {
			t.Errorf("ValidateSubCategory() error = %v, want nil", err)
		}
	})

	t.Run("invalid chain", func(t *testing.T) {
		orphan := Category{Code: "orphan", ParentCode: "does-not-exist"}
		if err := ValidateSubCategory(tax, "IN", orphan); err != ErrUnknownParent {
			t.Errorf("ValidateSubCategory() error = %v, want %v", err, ErrUnknownParent)
		}
	})
}
