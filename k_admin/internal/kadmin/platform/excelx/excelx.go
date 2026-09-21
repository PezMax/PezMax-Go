// Package excelx renders in-memory Excel (xlsx) exports with excelize,
// replacing the legacy Apache POI pipeline. Column sets follow the original
// Java export layout: one header row in entity field order, then one row per
// record — see the ptmj_file example in the package test for the datum file
// export contract (fileId → remark, in schema order).
package excelx

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize"
)

// MIMEType is the HTTP content type for xlsx payloads.
const MIMEType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

const defaultSheet = "Sheet1"

// Sheet describes one worksheet: a header row plus data rows. Cells accept
// anything excelize understands natively (string, ints, float64, bool,
// time.Time); nil renders as an empty cell.
type Sheet struct {
	Name    string
	Headers []string
	Rows    [][]interface{}
}

// Build renders the sheets into xlsx bytes.
func Build(sheets ...Sheet) ([]byte, error) {
	if len(sheets) == 0 {
		return nil, errors.New("excelx: at least one sheet is required")
	}
	file := excelize.NewFile()
	for index, sheet := range sheets {
		name := sheet.Name
		if strings.TrimSpace(name) == "" {
			if index == 0 {
				name = defaultSheet
			} else {
				return nil, fmt.Errorf("excelx: sheet %d needs a name", index)
			}
		}
		if index > 0 {
			file.NewSheet(name)
		} else if name != defaultSheet {
			file.SetSheetName(defaultSheet, name)
		}
		if len(sheet.Headers) == 0 {
			return nil, fmt.Errorf("excelx: sheet %q needs header columns", name)
		}
		for columnIndex, header := range sheet.Headers {
			cell, err := coordinates(columnIndex, 1)
			if err != nil {
				return nil, err
			}
			file.SetCellValue(name, cell, header)
			file.SetColWidth(name, columnLetter(columnIndex), columnLetter(columnIndex), 16)
		}
		for rowIndex, row := range sheet.Rows {
			if len(row) > len(sheet.Headers) {
				return nil, fmt.Errorf("excelx: sheet %q row %d has %d cells, want at most %d", name, rowIndex+2, len(row), len(sheet.Headers))
			}
			for columnIndex, value := range row {
				cell, err := coordinates(columnIndex, rowIndex+2)
				if err != nil {
					return nil, err
				}
				file.SetCellValue(name, cell, value)
			}
		}
	}
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, fmt.Errorf("excelx: write workbook: %w", err)
	}
	return buffer.Bytes(), nil
}

// ExportFilename renders the RuoYi-style download name: prefix_YYYYMMDDHHMMSS.xlsx.
func ExportFilename(prefix string, now time.Time) string {
	return prefix + "_" + now.Format("20060102150405") + ".xlsx"
}

func coordinates(column, row int) (string, error) {
	if column < 0 || column > 255 {
		return "", fmt.Errorf("excelx: column index %d out of range", column)
	}
	if row < 1 {
		return "", fmt.Errorf("excelx: row index %d out of range", row)
	}
	return fmt.Sprintf("%s%d", columnLetter(column), row), nil
}

func columnLetter(index int) string {
	letters := ""
	for ; index >= 0; index = index/26 - 1 {
		letters = string(rune('A'+index%26)) + letters
		if index < 26 {
			break
		}
	}
	return letters
}
