package sq

import "bytes"

type querylite struct {
	fields     Fields
	writeQuery string
	readQuery  string
	args       []interface{}
}

// TODO: remove hardcoded selectFields of querylite, use fieldliterals instead.
// That way GetFetchableFields on querylite will not be a dud, and querylite
// can play nice with CTE.Initial which invokes GetFetchableFields in the event
// that a column list was not provided.
func fieldliterals(fields ...string) []Field {
	fs := make([]Field, len(fields))
	for i := range fields {
		fs[i] = FieldLiteral(fields[i])
	}
	return fs
}

func (q querylite) AppendSQL(dialect string, buf *bytes.Buffer, args *[]interface{}, params map[string]int) error {
	var err error
	if q.readQuery != "" {
		if len(q.fields) > 0 {
			buf.WriteString("SELECT ")
			err = q.fields.AppendSQLExcludeWithAlias(dialect, buf, args, make(map[string]int), nil)
			if err != nil {
				return err
			}
			buf.WriteString(" ")
		}
		buf.WriteString(q.readQuery)
		*args = append(*args, q.args...)
		return nil
	}
	if q.writeQuery != "" {
		buf.WriteString(q.writeQuery)
		*args = append(*args, q.args...)
		err = q.fields.AppendSQLExcludeWithAlias(dialect, buf, args, make(map[string]int), nil)
		if err != nil {
			return err
		}
		if len(q.fields) > 0 {
			buf.WriteString(" RETURNING ")
			err = q.fields.AppendSQLExcludeWithAlias(dialect, buf, args, make(map[string]int), nil)
			if err != nil {
				return err
			}
		}
		return nil
	}
	buf.WriteString("SELECT ")
	err = q.fields.AppendSQLExcludeWithAlias(dialect, buf, args, make(map[string]int), nil)
	if err != nil {
		return err
	}
	return nil
}
func (q querylite) ToSQL() (query string, args []interface{}, params map[string]int, err error) {
	buf := &bytes.Buffer{}
	params = make(map[string]int)
	err = q.AppendSQL("", buf, &args, params)
	if err != nil {
		return buf.String(), args, params, err
	}
	return buf.String(), args, params, nil
}
func (q querylite) SetFetchableFields(fields []Field) (Query, error) {
	q.fields = fields
	return q, nil
}
func (q querylite) GetFetchableFields() ([]Field, error) {
	return q.fields, nil
}
func (q querylite) Dialect() string { return "sqlite3" }
