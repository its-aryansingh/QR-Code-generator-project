package httpapi

import (
	"encoding/json"
	"testing"
)

func TestSCIMFilterParser(t *testing.T) {
	cl, err := parseSCIMFilter(`userName eq "a@b.com" and externalId eq "x\"y"`)
	if err != nil || len(cl) != 2 || cl[0].attr != "username" || cl[1].value != `x"y` {
		t.Fatalf("%+v %v", cl, err)
	}
	cl, err = parseSCIMFilter(`emails[type eq "work"].value eq "a@b.com"`)
	if err != nil || cl[0].attr != `emails[typeeq"work"].value` {
		t.Fatalf("%+v %v", cl, err)
	}
	for _, bad := range []string{`userName co "a"`, `userName eq a`, `(userName eq "a")`, `userName eq "a" or id eq "b"`} {
		if _, err := parseSCIMFilter(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSCIMUserPatch(t *testing.T) {
	res := map[string]any{"userName": "a@b.com", "active": true, "emails": []any{map[string]any{"type": "work", "value": "a@b.com"}}}
	ops := []patchOp{
		{Op: "Replace", Path: "ACTIVE", Value: json.RawMessage(`"False"`)},
		{Op: "Add", Path: "name.familyName", Value: json.RawMessage(`"Lee"`)},
		{Op: "replace", Path: `emails[type eq "work"].value`, Value: json.RawMessage(`"new@b.com"`)},
		{Op: "add", Path: `emails[type eq "home"].value`, Value: json.RawMessage(`"home@b.com"`)},
		{Op: "add", Path: scimEntSchema + ":manager", Value: json.RawMessage(`{"value":"m1"}`)},
		{Op: "replace", Value: json.RawMessage(`{"displayName":"A Lee","title":"CTO"}`)},
		{Op: "remove", Path: "title"},
	}
	if err := applyUserPatch(res, ops); err != nil {
		t.Fatal(err)
	}
	if res["active"] != false || dig(res, "name", "familyName") != "Lee" || res["displayName"] != "A Lee" || res["title"] != nil {
		t.Fatalf("%v", res)
	}
	_, email, _, family, _, _, active := userFields(res)
	if email != "new@b.com" || family != "Lee" || active {
		t.Fatalf("fields: %s %s %v", email, family, active)
	}
	if len(res["emails"].([]any)) != 2 || dig(res, scimEntSchema, "manager", "value") != "m1" {
		t.Fatalf("emails/ext: %v", res)
	}
	if err := applyUserPatch(res, []patchOp{{Op: "copy", Path: "x"}}); err == nil {
		t.Fatal("unsupported op accepted")
	}
	if err := applyUserPatch(res, []patchOp{{Op: "replace", Value: json.RawMessage(`"str"`)}}); err == nil {
		t.Fatal("pathless non-object accepted")
	}
}
