package policy

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/ext"
	"google.golang.org/protobuf/encoding/prototext"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

type positionChecker struct {
	t  *testing.T
	si *exprpb.SourceInfo
}

func (v *positionChecker) VisitExpr(e ast.Expr) {
	if e.AsCall() != nil && e.AsCall().FunctionName() == "_?_:_" {
		// Ignore ternary operator as it's a synthetic node.
		return
	}
	if _, found := v.si.Positions[e.ID()]; !found {
		pbExpr, _ := ast.ExprToProto(e)
		v.t.Errorf("No position found for expression ID: %d, %s", e.ID(), prototext.Format(pbExpr))
	}
}

func (v *positionChecker) VisitEntryExpr(e ast.EntryExpr) {
	if _, found := v.si.Positions[e.ID()]; !found {
		v.t.Errorf("No position found for expression ID: %d", e.ID())
	}
}

func TestCompose_SourceInfo(t *testing.T) {
	policyYAML := `name: test_policy
rule:
  match:
    - condition: "2 == 1"
      output: "'hi'"
    - output: "'hello' + ' world'"
`
	src := StringSource(policyYAML, "test_policy.yaml")
	parser, err := NewParser()
	if err != nil {
		t.Fatalf("NewParser() failed: %v", err)
	}
	policy, iss := parser.Parse(src)
	if iss.Err() != nil {
		t.Fatalf("parser.Parse() failed: %v", iss.Err())
	}

	env, err := cel.NewEnv(cel.OptionalTypes(), ext.Bindings())
	if err != nil {
		t.Fatalf("cel.NewEnv() failed: %v", err)
	}
	compiledRule, iss := CompileRule(env, policy)
	if iss.Err() != nil {
		t.Fatalf("CompileRule() failed: %v", iss.Err())
	}
	composer, err := NewRuleComposer(env)
	if err != nil {
		t.Fatalf("NewRuleComposer() failed: %v", err)
	}
	compAST, iss := composer.Compose(compiledRule)
	if iss.Err() != nil {
		t.Fatalf("composer.Compose() failed: %v", iss.Err())
	}

	si := compAST.SourceInfo()
	if si.Location != "test_policy.yaml" {
		t.Errorf("SourceInfo.Location got %q, wanted test_policy.yaml", si.Location)
	}
	checker := &positionChecker{t: t, si: si}
	ast.PostOrderVisit(compAST.NativeRep().Expr(), checker)
}
