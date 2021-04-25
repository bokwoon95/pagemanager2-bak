package sq

import "bytes"

type Assignment struct {
	LHS interface{}
	RHS interface{}
}

func Assign(LHS, RHS interface{}) Assignment {
	return Assignment{LHS: LHS, RHS: RHS}
}

func (a Assignment) AppendSQLExclude(dialect string, buf *bytes.Buffer, args *[]interface{}, params map[string]int, excludedTableQualifiers []string) error {
	err := appendSQLValue(buf, args, params, excludedTableQualifiers, a.LHS)
	if err != nil {
		return err
	}
	buf.WriteString(" = ")
	switch a.RHS.(type) {
	case Query:
		buf.WriteString("(")
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, a.RHS)
		if err != nil {
			return err
		}
		buf.WriteString(")")
	default:
		err = appendSQLValue(buf, args, params, excludedTableQualifiers, a.RHS)
		if err != nil {
			return err
		}
	}
	return nil
}

type Assignments []Assignment

func (as Assignments) AppendSQLExclude(dialect string, buf *bytes.Buffer, args *[]interface{}, params map[string]int, excludedTableQualifiers []string) error {
	var err error
	for i, a := range as {
		if i > 0 {
			buf.WriteString(", ")
		}
		err = a.AppendSQLExclude(dialect, buf, args, params, excludedTableQualifiers)
		if err != nil {
			return err
		}
	}
	return nil
}

func SetExcluded(field Field) Assignment {
	name := field.GetName()
	return Assignment{LHS: FieldLiteral(name), RHS: FieldLiteral("EXCLUDED." + name)}
}
