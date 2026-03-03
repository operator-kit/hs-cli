package output

import (
	"encoding/csv"
	"io"
)

type CSVFormatter struct{}

func (f *CSVFormatter) Format(w io.Writer, columns []string, rows []map[string]string) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			record[i] = row[col]
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	return nil
}
