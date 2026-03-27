package documentparser

import (
	"fmt"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExtractExcelAllSheets reads every sheet as labeled CSV-style rows.
func ExtractExcelAllSheets(path string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return nil, fmt.Errorf("Excel file has no sheets")
	}

	var meta []string
	for _, name := range sheetList {
		rows, err := f.GetRows(name)
		rowCount := 0
		if err == nil {
			rowCount = len(rows)
		}
		meta = append(meta, fmt.Sprintf("Sheet %q: %d rows", name, rowCount))
	}

	var lines []string
	lines = append(lines, "Excel workbook — data as CSV-style rows per sheet.")

	for _, sheetName := range sheetList {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			lines = append(lines, fmt.Sprintf("--- Sheet %q (error reading rows: %v) ---", sheetName, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("--- Sheet: %s (%d rows) ---", sheetName, len(rows)))
		for i, row := range rows {
			csvLine := FormatRowAsCSV(row)
			lines = append(lines, fmt.Sprintf("%d: %s", i+1, csvLine))
		}
	}

	return &Result{
		Format:     "excel",
		FileSizeMB: fileSizeMB,
		Lines:      lines,
		Meta:       meta,
	}, nil
}

// ExtractExcelSheet returns rows for one sheet (CSV-style lines), with sheet list in Meta.
func ExtractExcelSheet(path, sheetName string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSizeMB := float64(stat.Size()) / (1024 * 1024)

	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return nil, fmt.Errorf("Excel file has no sheets")
	}

	var meta []string
	for _, name := range sheetList {
		rows, err := f.GetRows(name)
		rowCount := 0
		if err == nil {
			rowCount = len(rows)
		}
		meta = append(meta, fmt.Sprintf("Sheet %q: %d rows", name, rowCount))
	}

	targetSheet := sheetName
	if targetSheet == "" {
		targetSheet = sheetList[0]
	}

	sheetExists := false
	for _, name := range sheetList {
		if name == targetSheet {
			sheetExists = true
			break
		}
	}
	if !sheetExists {
		return &Result{
			Format:     "excel",
			FileSizeMB: fileSizeMB,
			Meta:       meta,
			Lines: []string{
				fmt.Sprintf("Unknown sheet %q. Available: %v", targetSheet, sheetList),
			},
		}, nil
	}

	rows, err := f.GetRows(targetSheet)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %q: %w", targetSheet, err)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("--- Sheet: %s (%d rows) ---", targetSheet, len(rows)))
	for i, row := range rows {
		lines = append(lines, fmt.Sprintf("%d: %s", i+1, FormatRowAsCSV(row)))
	}

	return &Result{
		Format:     "excel",
		FileSizeMB: fileSizeMB,
		Lines:      lines,
		Meta:       meta,
	}, nil
}

// FormatRowAsCSV renders one spreadsheet row as CSV with quoting when needed.
func FormatRowAsCSV(row []string) string {
	var parts []string
	for _, cell := range row {
		if strings.ContainsAny(cell, ",\"\n\r") {
			cell = "\"" + strings.ReplaceAll(cell, "\"", "\"\"") + "\""
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, ",")
}
