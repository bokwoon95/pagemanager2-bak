package sq

import (
	"fmt"
	"strings"
)

type PredicateCase struct {
	condition Predicate
	result    interface{}
}

type PredicateCases struct {
	alias    string
	cases    []PredicateCase
	fallback interface{}
}

func (f PredicateCases) GetAlias() string { return f.alias }

func (f PredicateCases) GetName() string { return "" }

func (f PredicateCases) AppendSQLExclude(dialect string, buf *strings.Builder, args *[]interface{}, params map[string]int, excludedTableQualifiers []string) error {
	buf.WriteString("CASE")
	var err error
	for _, Case := range f.cases {
		buf.WriteString(" WHEN ")
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, Case.condition)
		if err != nil {
			return err
		}
		buf.WriteString(" THEN ")
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, Case.result)
		if err != nil {
			return err
		}
	}
	if len(f.cases) == 0 {
		return fmt.Errorf("no predicate cases provided")
	}
	if f.fallback != nil {
		buf.WriteString(" ELSE ")
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, f.fallback)
		if err != nil {
			return err
		}
	}
	buf.WriteString(" END")
	return nil
}

func (f PredicateCases) As(alias string) PredicateCases {
	f.alias = alias
	return f
}

func CaseWhen(predicate Predicate, result interface{}) PredicateCases {
	f := PredicateCases{}
	f.cases = append(f.cases, PredicateCase{
		condition: predicate,
		result:    result,
	})
	return f
}

func (f PredicateCases) When(predicate Predicate, result interface{}) PredicateCases {
	f.cases = append(f.cases, PredicateCase{
		condition: predicate,
		result:    result,
	})
	return f
}

func (f PredicateCases) Else(fallback interface{}) PredicateCases {
	f.fallback = fallback
	return f
}

type SimpleCase struct {
	value  interface{}
	result interface{}
}

type SimpleCases struct {
	alias      string
	expression interface{}
	cases      []SimpleCase
	fallback   interface{}
}

func (f SimpleCases) GetAlias() string { return f.alias }

func (f SimpleCases) GetName() string { return "" }

func (f SimpleCases) AppendSQLExclude(dialect string, buf *strings.Builder, args *[]interface{}, params map[string]int, excludedTableQualifiers []string) error {
	buf.WriteString("CASE ")
	err := appendSQLValue(buf, args, params, excludedTableQualifiers, f.expression)
	if err != nil {
		return err
	}
	for _, Case := range f.cases {
		buf.WriteString(" WHEN ")
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, Case.value)
		if err != nil {
			return err
		}
		buf.WriteString(" THEN ")
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, Case.result)
		if err != nil {
			return err
		}
	}
	if len(f.cases) == 0 {
		return fmt.Errorf("no predicate cases provided")
	}
	if f.fallback != nil {
		buf.WriteString(" ELSE ")
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, f.fallback)
		if err != nil {
			return err
		}
	}
	buf.WriteString(" END")
	return nil
}

func (f SimpleCases) As(alias string) SimpleCases {
	f.alias = alias
	return f
}

func Case(field Field) SimpleCases { return SimpleCases{expression: field} }

func (f SimpleCases) When(value interface{}, result interface{}) SimpleCases {
	f.cases = append(f.cases, SimpleCase{
		value:  value,
		result: result,
	})
	return f
}

func (f SimpleCases) Else(fallback interface{}) SimpleCases {
	f.fallback = fallback
	return f
}
