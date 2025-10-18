package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

type DPRDownloadHandler struct {
	db *sql.DB
}

func NewDPRDownloadHandler(db *sql.DB) *DPRDownloadHandler {
	return &DPRDownloadHandler{db: db}
}

// DownloadDPRDKabCalegByProvince handles Excel download for DPRD Kab caleg data by province (multi-sheet per dapil)
func (h *DPRDownloadHandler) DownloadDPRDKabCalegByProvince(c echo.Context) error {
	proKode := c.Param("id")

	// Get province info
	var proID, proNama string
	err := h.db.QueryRow("SELECT pro_id, pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proKode).Scan(&proID, &proNama)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all dapils in this province from hr_dprd_kab_kec
	dapilQuery := `
		SELECT DISTINCT dapil_kode, dapil_nama, kab_kode, kab_nama
		FROM hr_dprd_kab_kec
		WHERE pro_kode = ?
		ORDER BY dapil_kode
	`
	dapilRows, err := h.db.Query(dapilQuery, proKode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying dapil data")
	}
	defer dapilRows.Close()

	type DapilInfo struct {
		DapilKode string
		DapilNama string
		KabKode   string
		KabNama   string
	}

	var dapilList []DapilInfo
	for dapilRows.Next() {
		var d DapilInfo
		if err := dapilRows.Scan(&d.DapilKode, &d.DapilNama, &d.KabKode, &d.KabNama); err != nil {
			continue
		}
		dapilList = append(dapilList, d)
	}

	if len(dapilList) == 0 {
		return c.String(http.StatusNotFound, "Tidak ada data dapil untuk provinsi ini")
	}

	// Create Excel file
	f := excelize.NewFile()

	// Process each dapil
	firstSheet := true
	for _, dapil := range dapilList {
		// Sanitize sheet name (max 31 chars, no special chars)
		sheetName := sanitizeSheetName(dapil.DapilNama)

		if firstSheet {
			// Rename Sheet1 to first dapil name
			f.SetSheetName("Sheet1", sheetName)
			firstSheet = false
		} else {
			_, err := f.NewSheet(sheetName)
			if err != nil {
				continue
			}
		}

		// Get kecamatan data for this dapil
		kecQuery := `
			SELECT DISTINCT
				kec_kode, kec_nama, kab_kode, kab_nama
			FROM hr_dprd_kab_kec
			WHERE dapil_kode = ? AND pro_kode = ?
			ORDER BY kec_kode
		`
		kecRows, err := h.db.Query(kecQuery, dapil.DapilKode, proKode)
		if err != nil {
			continue
		}

		type KecInfo struct {
			KecKode string
			KecNama string
			KabKode string
			KabNama string
		}

		var kecList []KecInfo
		for kecRows.Next() {
			var k KecInfo
			if err := kecRows.Scan(&k.KecKode, &k.KecNama, &k.KabKode, &k.KabNama); err != nil {
				continue
			}
			kecList = append(kecList, k)
		}
		kecRows.Close()

		// Get caleg list for this dapil
		calegQuery := `
			SELECT id, nama, nomor_urut, partai_id
			FROM dprd_kab_caleg
			WHERE dapil_kode = ?
			ORDER BY nomor_urut
		`
		calegRows, err := h.db.Query(calegQuery, dapil.DapilKode)
		if err != nil {
			continue
		}

		type Caleg struct {
			ID        string
			Nama      string
			NomorUrut int
			PartaiID  string
		}

		var calegList []Caleg
		for calegRows.Next() {
			var cal Caleg
			if err := calegRows.Scan(&cal.ID, &cal.Nama, &cal.NomorUrut, &cal.PartaiID); err != nil {
				continue
			}
			calegList = append(calegList, cal)
		}
		calegRows.Close()

		// Build headers
		headers := []string{
			"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
			"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		}

		// Add caleg columns
		for _, caleg := range calegList {
			headers = append(headers, fmt.Sprintf("%d. %s", caleg.NomorUrut, caleg.Nama))
		}
		headers = append(headers, "TOTAL")

		// Write headers
		for i, header := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, header)
		}

		// Style headers
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 11},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
			Border: []excelize.Border{
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
				{Type: "right", Color: "000000", Style: 1},
			},
		})
		lastCol, _ := excelize.ColumnNumberToName(len(headers))
		f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

		// Set column widths
		f.SetColWidth(sheetName, "A", "A", 5)
		f.SetColWidth(sheetName, "B", "D", 20)
		f.SetColWidth(sheetName, "E", "I", 15)

		// Write data rows
		rowNum := 2
		for idx, kec := range kecList {
			colNum := 1

			// NO
			cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, idx+1)
			colNum++

			// PROVINSI
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, proNama)
			colNum++

			// KODE PROV
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, proKode)
			colNum++

			// DAPIL
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, dapil.DapilNama)
			colNum++

			// KODE DAPIL
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, dapil.DapilKode)
			colNum++

			// KAB/KOTA
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kec.KabNama)
			colNum++

			// KODE KAB
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kec.KabKode)
			colNum++

			// KECAMATAN
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kec.KecNama)
			colNum++

			// KODE KEC
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kec.KecKode)
			colNum++

			// Get vote data from hr_dprd_kab_kec.tbl JSON field
			var tblJSON string
			err := h.db.QueryRow(`
				SELECT tbl
				FROM hr_dprd_kab_kec
				WHERE dapil_kode = ? AND kec_kode = ?
				LIMIT 1
			`, dapil.DapilKode, kec.KecKode).Scan(&tblJSON)

			// Parse tbl JSON to get caleg votes
			calegVotes := make(map[string]int)
			if err == nil && tblJSON != "" {
				var tblData map[string]map[string]interface{}
				if err := json.Unmarshal([]byte(tblJSON), &tblData); err == nil {
					// tblData contains kode_desa as keys, then caleg_id as nested keys
					for _, desaData := range tblData {
						for calegID, suaraVal := range desaData {
							if calegID != "null" {
								if suara, ok := suaraVal.(float64); ok {
									calegVotes[calegID] += int(suara)
								}
							}
						}
					}
				}
			}

			// Write caleg vote data
			totalSuara := 0
			for _, caleg := range calegList {
				cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
				suara := calegVotes[caleg.ID]
				f.SetCellValue(sheetName, cell, suara)
				totalSuara += suara
				colNum++
			}

			// TOTAL
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, totalSuara)

			rowNum++
		}
	}

	// Set first sheet as active
	if len(dapilList) > 0 {
		f.SetActiveSheet(0)
	}

	// Set filename
	filename := fmt.Sprintf("DPRD_Kabupaten_Caleg_%s_%s.xlsx", proKode, proNama)
	filename = strings.ReplaceAll(filename, " ", "_")
	filename = strings.ReplaceAll(filename, "/", "_")

	// Set response headers
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Write to response
	return f.Write(c.Response().Writer)
}

// sanitizeSheetName cleans sheet name to be Excel-compatible
func sanitizeSheetName(name string) string {
	// Remove invalid characters
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")

	// Truncate to 31 characters (Excel limit)
	if len(name) > 31 {
		name = name[:31]
	}

	return name
}

// DownloadDPRRIPartai handles Excel download for DPR RI party data by dapil
func (h *DPRDownloadHandler) DownloadDPRRIPartai(c echo.Context) error {
	code := c.Param("id")

	// Check if code is a dapil_kode first (prioritize dapil since URL is /download/dapil/...)
	var kabKode, kabNama, dapilID, dapilName, proCode, proName string

	// Try as dapil code first (kab_kode = '0' means it's the dapil master record)
	err := h.db.QueryRow("SELECT dapil_id, dapil_nama, pro_kode FROM pdpr_wil_dapil WHERE dapil_kode = ? AND kab_kode = '0'", code).Scan(&dapilID, &dapilName, &proCode)
	if err == nil {
		// It's a dapil code

		// Get first kabupaten in this dapil
		err = h.db.QueryRow("SELECT DISTINCT kab_kode, kab_nama FROM pdpr_wil_kel WHERE dapil_id = ? ORDER BY kab_kode LIMIT 1", dapilID).Scan(&kabKode, &kabNama)
		if err != nil {
			return c.String(http.StatusNotFound, "Kabupaten dalam dapil tidak ditemukan")
		}
	} else {
		// Try as kabupaten code
		err = h.db.QueryRow("SELECT kab_kode, kab_nama, pro_kode FROM pdpr_wil_kab WHERE kab_kode = ?", code).Scan(&kabKode, &kabNama, &proCode)
		if err != nil {
			return c.String(http.StatusNotFound, "Dapil/Kabupaten tidak ditemukan")
		}

		// Get dapil info from kelurahan (take first dapil in this kabupaten)
		err = h.db.QueryRow("SELECT DISTINCT dapil_id, dapil_nama FROM pdpr_wil_kel WHERE kab_kode = ? LIMIT 1", kabKode).Scan(&dapilID, &dapilName)
		if err != nil {
			return c.String(http.StatusNotFound, "Dapil untuk kabupaten tidak ditemukan")
		}
	}

	// Get province name
	err = h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = "Unknown"
	}

	// Get all kelurahan in this dapil (all kabupaten in the dapil)
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama
		FROM pdpr_wil_kel k
		JOIN pdpr_wil_kec kec ON k.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON kec.kab_kode = kab.kab_kode
		WHERE k.dapil_id = ?
		ORDER BY kab.kab_nama, kec.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelurahanQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data")
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
	}

	var kelurahanList []KelurahanInfo
	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
	}

	// Get TPS count and DPT sum per kelurahan
	tpsQuery := `SELECT kel_kode, COUNT(*) as jml_tps, SUM(COALESCE(total_dpt, 0)) as jml_dpt FROM pdpr_wil_tps WHERE dapil_id = ? GROUP BY kel_kode`

	tpsRows, err := h.db.Query(tpsQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS, jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err != nil {
			continue
		}
		tpsData[kelKode] = struct {
			JmlTPS int
			JmlDPT int
		}{jmlTPS, jmlDPT}
	}

	// Get vote data from hr_dpr_ri_kel (per TPS, need to aggregate per kelurahan)
	voteDataQuery := `SELECT kel_kode, tbl, chart FROM hr_dpr_ri_kel WHERE dapil_id = ?`

	voteRows, err := h.db.Query(voteDataQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Parse party data from hr_dpr_ri_kel.chart
	partaiData := make(map[string]map[int]struct {
		JmlSuaraTotal  int
		JmlSuaraPartai int
	})

	for voteRows.Next() {
		var kelKode, tblJSON, chartJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON, &chartJSON); err != nil {
			continue
		}

		// Parse party data: {"1": {"jml_suara_total": 38, "jml_suara_partai": 4}, ...}
		var partaiDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(chartJSON), &partaiDataMap); err != nil {
			continue
		}

		if _, exists := partaiData[kelKode]; !exists {
			partaiData[kelKode] = make(map[int]struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			})
		}

		for nomorUrutStr, data := range partaiDataMap {
			nomorUrut, _ := strconv.Atoi(nomorUrutStr)
			jmlSuaraTotal := 0
			jmlSuaraPartai := 0

			if val, ok := data["jml_suara_total"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraTotal = int(v)
				case int:
					jmlSuaraTotal = v
				case string:
					jmlSuaraTotal, _ = strconv.Atoi(v)
				}
			}

			if val, ok := data["jml_suara_partai"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraPartai = int(v)
				case int:
					jmlSuaraPartai = v
				case string:
					jmlSuaraPartai, _ = strconv.Atoi(v)
				}
			}

			partaiData[kelKode][nomorUrut] = struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			}{jmlSuaraTotal, jmlSuaraPartai}
		}
	}

	// Get unique parties with their nomor_urut
	partaiQuery := `
		SELECT DISTINCT p.id, p.nama, p.partai_singkat, p.nomor_urut
		FROM partai p
		INNER JOIN dpr_ri_caleg c ON p.id = c.partai_id
		WHERE c.dapil_id = ?
		ORDER BY p.nomor_urut
	`
	partaiRows, err := h.db.Query(partaiQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying party data")
	}
	defer partaiRows.Close()

	type Partai struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}

	var partaiList []Partai
	for partaiRows.Next() {
		var p Partai
		if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
			continue
		}
		partaiList = append(partaiList, p)
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Partai DPR RI"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Set header - format standar
	headers := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Add party columns using short names
	for _, partai := range partaiList {
		headers = append(headers, partai.PartaiSingkat)
	}

	// Add TOTAL column
	headers = append(headers, "TOTAL")

	// Write headers
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(len(headers))
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 20)
	f.SetColWidth(sheetName, "D", "D", 25)

	// Write data rows
	rowNum := 2
	for idx, kel := range kelurahanList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.ProKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilName)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelKode)
		colNum++

		// TPS (jumlah TPS untuk data perdesa)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		jmlTPS := 0
		jmlDPT := 0
		if tpsInfo, exists := tpsData[kel.KelKode]; exists {
			jmlTPS = tpsInfo.JmlTPS
			jmlDPT = tpsInfo.JmlDPT
		}
		f.SetCellValue(sheetName, cell, jmlTPS)
		colNum++

		// KODE TPS (kosong untuk data perdesa/kelurahan)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, "")
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, jmlDPT)
		colNum++

		// Party votes - hanya jml_suara_total
		totalSuara := 0
		for _, partai := range partaiList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)

			// Check if data exists for this kelurahan and party
			kelPartaiData, kelExists := partaiData[kel.KelKode]
			if kelExists {
				if data, partaiExists := kelPartaiData[partai.NomorUrut]; partaiExists {
					// Data exists, use the actual value (could be 0)
					f.SetCellValue(sheetName, cell, data.JmlSuaraTotal)
					totalSuara += data.JmlSuaraTotal
				} else {
					// Party exists but no data for this party
					f.SetCellValue(sheetName, cell, "-")
				}
			} else {
				// No data for this kelurahan at all
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if _, kelExists := partaiData[kel.KelKode]; kelExists {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	lastRow := rowNum - 1
	f.SetCellStyle(sheetName, "A2", lastCol+fmt.Sprintf("%d", lastRow), dataStyle)

	// Set response headers
	filename := fmt.Sprintf("DPR_RI_%s_%s.xlsx", proName, dapilName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Write to response
	return f.Write(c.Response().Writer)
}

// DownloadDPRRICaleg handles Excel download for DPR RI caleg (candidate) data by dapil
func (h *DPRDownloadHandler) DownloadDPRRICaleg(c echo.Context) error {
	code := c.Param("id")

	// Get dapil info - try as dapil_kode first, then as kab_kode
	var dapilID, dapilName, proCode, kabKode string
	var dapilIDs []string
	var isKabKode bool
	err := h.db.QueryRow("SELECT dapil_id, dapil_nama, pro_kode FROM pdpr_wil_dapil WHERE dapil_kode = ?", code).Scan(&dapilID, &dapilName, &proCode)
	if err != nil {
		// Try to find by kab_kode - get ALL dapil_ids for this kabupaten
		isKabKode = true
		kabKode = code

		// First check if kabupaten exists and get province info
		var kabNama string
		err = h.db.QueryRow("SELECT kab_nama, pro_kode FROM pdpr_wil_kab WHERE kab_kode = ?", code).Scan(&kabNama, &proCode)
		if err != nil {
			return c.String(http.StatusNotFound, "Kabupaten tidak ditemukan")
		}

		// Get all dapil_ids that cover this kabupaten
		dapilRows, err := h.db.Query("SELECT DISTINCT dapil_id FROM pdpr_wil_kel WHERE kab_kode = ?", kabKode)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Error querying dapil data")
		}
		defer dapilRows.Close()

		for dapilRows.Next() {
			var did string
			if err := dapilRows.Scan(&did); err == nil {
				dapilIDs = append(dapilIDs, did)
			}
		}

		if len(dapilIDs) == 0 {
			return c.String(http.StatusNotFound, "Dapil tidak ditemukan untuk kabupaten ini")
		}

		// Use first dapil_id for compatibility
		dapilID = dapilIDs[0]
		dapilName = kabNama
	} else {
		dapilIDs = []string{dapilID}
	}

	// Get province name
	var proName string
	err = h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = "Unknown"
	}

	// Get all kelurahan in this dapil (or specific kabupaten if kab_kode is provided)
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama
		FROM pdpr_wil_kel k
		JOIN pdpr_wil_kec kec ON k.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON kec.kab_kode = kab.kab_kode
		WHERE 1=1`

	if isKabKode {
		kelurahanQuery += ` AND k.kab_kode = ?`
	} else {
		kelurahanQuery += ` AND k.dapil_id = ?`
	}

	kelurahanQuery += `
		ORDER BY kab.kab_nama, kec.kec_nama, k.kel_nama
	`

	var kelRows *sql.Rows
	if isKabKode {
		kelRows, err = h.db.Query(kelurahanQuery, kabKode)
	} else {
		kelRows, err = h.db.Query(kelurahanQuery, dapilID)
	}
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data")
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
	}

	var kelurahanList []KelurahanInfo
	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
	}

	// Get TPS count and DPT sum per kelurahan
	tpsQuery := `SELECT kel_kode, COUNT(*) as jml_tps, SUM(COALESCE(total_dpt, 0)) as jml_dpt FROM pdpr_wil_tps WHERE 1=1`
	if isKabKode {
		tpsQuery += ` AND kab_kode = ?`
	} else {
		tpsQuery += ` AND dapil_id = ?`
	}
	tpsQuery += ` GROUP BY kel_kode`

	var tpsRows *sql.Rows
	if isKabKode {
		tpsRows, err = h.db.Query(tpsQuery, kabKode)
	} else {
		tpsRows, err = h.db.Query(tpsQuery, dapilID)
	}
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS, jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err != nil {
			continue
		}
		tpsData[kelKode] = struct {
			JmlTPS int
			JmlDPT int
		}{jmlTPS, jmlDPT}
	}

	// Get vote data from hr_dpr_ri_kel (per TPS, need to aggregate per kelurahan)
	voteDataQuery := `SELECT kel_kode, tbl, chart FROM hr_dpr_ri_kel WHERE 1=1`
	if isKabKode {
		voteDataQuery += ` AND kab_kode = ?`
	} else {
		voteDataQuery += ` AND dapil_id = ?`
	}

	var voteRows *sql.Rows
	if isKabKode {
		voteRows, err = h.db.Query(voteDataQuery, kabKode)
	} else {
		voteRows, err = h.db.Query(voteDataQuery, dapilID)
	}
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Aggregate votes per kelurahan
	chartData := make(map[string]map[string]int)
	partaiData := make(map[string]map[int]struct {
		JmlSuaraTotal  int
		JmlSuaraPartai int
	})

	for voteRows.Next() {
		var kelKode, tblJSON, chartJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON, &chartJSON); err != nil {
			continue
		}

		// Parse TPS data: {"1101012001001": {"100121": 3, "100122": 1, ...}, ...}
		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		// Aggregate all TPS votes for this kelurahan
		if _, exists := chartData[kelKode]; !exists {
			chartData[kelKode] = make(map[string]int)
		}

		for _, votes := range tpsDataMap {
			for calegID, voteVal := range votes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				chartData[kelKode][calegID] += votes
			}
		}

		// Parse party data: {"1": {"jml_suara_total": 38, "jml_suara_partai": 4}, ...}
		var partaiDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(chartJSON), &partaiDataMap); err != nil {
			continue
		}

		if _, exists := partaiData[kelKode]; !exists {
			partaiData[kelKode] = make(map[int]struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			})
		}

		for nomorUrutStr, data := range partaiDataMap {
			nomorUrut, _ := strconv.Atoi(nomorUrutStr)
			jmlSuaraTotal := 0
			jmlSuaraPartai := 0

			if val, ok := data["jml_suara_total"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraTotal = int(v)
				case int:
					jmlSuaraTotal = v
				case string:
					jmlSuaraTotal, _ = strconv.Atoi(v)
				}
			}

			if val, ok := data["jml_suara_partai"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraPartai = int(v)
				case int:
					jmlSuaraPartai = v
				case string:
					jmlSuaraPartai, _ = strconv.Atoi(v)
				}
			}

			partaiData[kelKode][nomorUrut] = struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			}{jmlSuaraTotal, jmlSuaraPartai}
		}
	}

	// Get all candidates for this dapil (or primary dapil for the kabupaten)
	var candidateQuery string
	var candRows *sql.Rows

	// Always use only the primary dapil (dapilID already set to first/primary dapil)
	candidateQuery = `
		SELECT c.id, c.nama, p.nama as nama_partai, p.partai_singkat, c.nomor_urut, c.partai_id
		FROM dpr_ri_caleg c
		LEFT JOIN partai p ON c.partai_id = p.id
		WHERE c.dapil_id = ?
		ORDER BY p.nomor_urut, c.nomor_urut
	`
	candRows, err = h.db.Query(candidateQuery, dapilID)

	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying candidate data")
	}
	defer candRows.Close()

	type Candidate struct {
		ID            string
		Nama          string
		Partai        string
		PartaiSingkat string
		NomorUrut     int
		PartaiID      int
	}

	var candidates []Candidate
	candidatesByPartai := make(map[int][]Candidate)
	for candRows.Next() {
		var c Candidate
		if err := candRows.Scan(&c.ID, &c.Nama, &c.Partai, &c.PartaiSingkat, &c.NomorUrut, &c.PartaiID); err != nil {
			continue
		}
		candidates = append(candidates, c)
		candidatesByPartai[c.PartaiID] = append(candidatesByPartai[c.PartaiID], c)
	}

	// Get unique parties with their nomor_urut
	var partaiQuery string
	var partaiRows *sql.Rows

	// Always use only the primary dapil
	partaiQuery = `
		SELECT DISTINCT p.id, p.nama, p.partai_singkat, p.nomor_urut
		FROM partai p
		INNER JOIN dpr_ri_caleg c ON p.id = c.partai_id
		WHERE c.dapil_id = ?
		ORDER BY p.nomor_urut
	`
	partaiRows, err = h.db.Query(partaiQuery, dapilID)

	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying party data")
	}
	defer partaiRows.Close()

	type Partai struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}

	var partaiList []Partai
	for partaiRows.Next() {
		var p Partai
		if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
			continue
		}
		partaiList = append(partaiList, p)
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Caleg DPR RI"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Fixed columns - format standar
	fixedCols := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Calculate total columns
	totalCols := len(fixedCols) + (len(partaiList) * 2) + len(candidates) + 1

	// Create 3-row headers
	colNum := 1

	// Write fixed columns headers - Row 1 and 2 will be merged
	// Set "partai" in A1 only (will be merged later)
	f.SetCellValue(sheetName, "A1", "partai")
	// Set "nomor urut caleg" in A2 only (will be merged later)
	f.SetCellValue(sheetName, "A2", "nomor urut caleg")

	// Write Row 3: column labels for fixed columns
	for idx, col := range fixedCols {
		cell3, _ := excelize.CoordinatesToCellName(idx+1, 3)
		f.SetCellValue(sheetName, cell3, col)
	}

	// Merge A1:Q1 (17 fixed columns)
	fixedColEnd, _ := excelize.ColumnNumberToName(len(fixedCols))
	f.MergeCell(sheetName, "A1", fixedColEnd+"1")
	// Merge A2:Q2
	f.MergeCell(sheetName, "A2", fixedColEnd+"2")

	colNum = len(fixedCols) + 1

	// Write party columns with candidates
	for _, partai := range partaiList {
		startCol := colNum // Track start column for merging

		// TOTAL_PARTAI column
		cell2, _ := excelize.CoordinatesToCellName(colNum, 2)
		f.SetCellValue(sheetName, cell2, "")
		cell3, _ := excelize.CoordinatesToCellName(colNum, 3)
		f.SetCellValue(sheetName, cell3, fmt.Sprintf("TOTAL_PARTAI_%d", partai.NomorUrut))
		colNum++

		// GAMBAR_PARTAI column
		cell2, _ = excelize.CoordinatesToCellName(colNum, 2)
		f.SetCellValue(sheetName, cell2, "")
		cell3, _ = excelize.CoordinatesToCellName(colNum, 3)
		f.SetCellValue(sheetName, cell3, fmt.Sprintf("GAMBAR_PARTAI_%d", partai.NomorUrut))
		colNum++

		// Write all candidates for this party
		if calegs, exists := candidatesByPartai[partai.ID]; exists {
			for _, cand := range calegs {
				cell2, _ := excelize.CoordinatesToCellName(colNum, 2)
				f.SetCellValue(sheetName, cell2, cand.NomorUrut)
				cell3, _ := excelize.CoordinatesToCellName(colNum, 3)
				f.SetCellValue(sheetName, cell3, cand.Nama)
				colNum++
			}
		}

		// Merge party name in row 1 across all columns for this party
		endCol := colNum - 1
		startCell, _ := excelize.CoordinatesToCellName(startCol, 1)
		endCell, _ := excelize.CoordinatesToCellName(endCol, 1)
		f.SetCellValue(sheetName, startCell, partai.PartaiSingkat)
		if startCol != endCol {
			f.MergeCell(sheetName, startCell, endCell)
		}
	}

	// TOTAL SUARA column
	for row := 1; row <= 3; row++ {
		cell, _ := excelize.CoordinatesToCellName(colNum, row)
		f.SetCellValue(sheetName, cell, "TOTAL SUARA")
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(totalCols)
	f.SetCellStyle(sheetName, "A1", lastCol+"3", headerStyle)

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 20)
	f.SetColWidth(sheetName, "D", "D", 25)

	// Write data rows
	rowNum := 4
	for idx, kel := range kelurahanList {
		colNum := 1

		// Get TPS and DPT data
		jmlTPS := 0
		jmlDPT := 0
		if tpsInfo, exists := tpsData[kel.KelKode]; exists {
			jmlTPS = tpsInfo.JmlTPS
			jmlDPT = tpsInfo.JmlDPT
		}

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.ProKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilName)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelKode)
		colNum++

		// TPS (count)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, jmlTPS)
		colNum++

		// KODE TPS (empty for perdesa)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, "")
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, jmlDPT)
		colNum++

		// Get votes for this kelurahan
		kelVotes := chartData[kel.KelKode]

		// Calculate total votes from all parties (sum of jml_suara_total from all 18 parties)
		totalVotes := 0
		if kelPartaiData, exists := partaiData[kel.KelKode]; exists {
			for _, data := range kelPartaiData {
				totalVotes += data.JmlSuaraTotal
			}
		}

		// Party votes and candidates grouped by party
		for _, partai := range partaiList {
			// TOTAL_PARTAI
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			totalPartai := 0
			gambarPartai := 0
			if kelPartaiData, exists := partaiData[kel.KelKode]; exists {
				if data, ok := kelPartaiData[partai.NomorUrut]; ok {
					totalPartai = data.JmlSuaraTotal
					gambarPartai = data.JmlSuaraPartai
				}
			}
			f.SetCellValue(sheetName, cell, totalPartai)
			colNum++

			// GAMBAR_PARTAI
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, gambarPartai)
			colNum++

			// Candidate votes for this party
			if calegs, exists := candidatesByPartai[partai.ID]; exists {
				for _, cand := range calegs {
					cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
					votes := 0
					if kelVotes != nil {
						if v, ok := kelVotes[cand.ID]; ok {
							votes = v
						}
					}
					f.SetCellValue(sheetName, cell, votes)
					colNum++
				}
			}
		}

		// Total
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, totalVotes)

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	lastRow := rowNum - 1
	f.SetCellStyle(sheetName, "A4", lastCol+fmt.Sprintf("%d", lastRow), dataStyle)

	// Set response headers
	filename := fmt.Sprintf("DPR_RI_Caleg_%s_%s.xlsx", proName, dapilName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Write to response
	return f.Write(c.Response().Writer)
}

// DownloadProvinsiDPRRIPartai handles Excel download for all DPR RI party data in a province
func (h *DPRDownloadHandler) DownloadProvinsiDPRRIPartai(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all kelurahan in this province
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdpr_wil_kel k
		JOIN pdpr_wil_kec kec ON k.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON kec.kab_kode = kab.kab_kode
		JOIN pdpr_wil_dapil d ON k.dapil_id = d.dapil_id
		WHERE k.pro_kode = ?
		ORDER BY d.dapil_nama, kab.kab_nama, kec.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelurahanQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data")
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var kelurahanList []KelurahanInfo
	dapilIDs := make(map[string]bool)
	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama, &k.DapilNama); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
		dapilIDs[k.DapilID] = true
	}

	// Get TPS count and DPT sum per kelurahan for this province
	tpsQuery := `SELECT kel_kode, COUNT(*) as jml_tps, SUM(COALESCE(total_dpt, 0)) as jml_dpt FROM pdpr_wil_tps WHERE pro_kode = ? GROUP BY kel_kode`

	tpsRows, err := h.db.Query(tpsQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS, jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err != nil {
			continue
		}
		tpsData[kelKode] = struct {
			JmlTPS int
			JmlDPT int
		}{jmlTPS, jmlDPT}
	}

	// Get vote data from hr_dpr_ri_kel for all dapils in province
	voteDataQuery := `SELECT kel_kode, tbl, chart FROM hr_dpr_ri_kel WHERE pro_kode = ?`

	voteRows, err := h.db.Query(voteDataQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Parse party data from hr_dpr_ri_kel.chart
	partaiData := make(map[string]map[int]struct {
		JmlSuaraTotal  int
		JmlSuaraPartai int
	})

	for voteRows.Next() {
		var kelKode, tblJSON, chartJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON, &chartJSON); err != nil {
			continue
		}

		var partaiDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(chartJSON), &partaiDataMap); err != nil {
			continue
		}

		if _, exists := partaiData[kelKode]; !exists {
			partaiData[kelKode] = make(map[int]struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			})
		}

		for nomorUrutStr, data := range partaiDataMap {
			nomorUrut, _ := strconv.Atoi(nomorUrutStr)
			jmlSuaraTotal := 0
			jmlSuaraPartai := 0

			if val, ok := data["jml_suara_total"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraTotal = int(v)
				case int:
					jmlSuaraTotal = v
				case string:
					jmlSuaraTotal, _ = strconv.Atoi(v)
				}
			}

			if val, ok := data["jml_suara_partai"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraPartai = int(v)
				case int:
					jmlSuaraPartai = v
				case string:
					jmlSuaraPartai, _ = strconv.Atoi(v)
				}
			}

			partaiData[kelKode][nomorUrut] = struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			}{jmlSuaraTotal, jmlSuaraPartai}
		}
	}

	// Get unique parties (using first dapil as reference)
	var firstDapilID string
	for dapilID := range dapilIDs {
		firstDapilID = dapilID
		break
	}

	partaiQuery := `
		SELECT DISTINCT p.id, p.nama, p.partai_singkat, p.nomor_urut
		FROM partai p
		INNER JOIN dpr_ri_caleg c ON p.id = c.partai_id
		WHERE c.dapil_id = ?
		ORDER BY p.nomor_urut
	`
	partaiRows, err := h.db.Query(partaiQuery, firstDapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying party data")
	}
	defer partaiRows.Close()

	type Partai struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}

	var partaiList []Partai
	for partaiRows.Next() {
		var p Partai
		if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
			continue
		}
		partaiList = append(partaiList, p)
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Partai DPR RI"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Set header
	headers := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL", "KAB/KOTA", "KODE KAB",
		"KECAMATAN", "KODE KEC", "KELURAHAN/DESA", "KODE DESA",
		"TPS", "KODE TPS", "DPT",
	}

	// Add party columns
	for _, partai := range partaiList {
		headers = append(headers, partai.PartaiSingkat)
	}

	// Add TOTAL column
	headers = append(headers, "TOTAL")

	// Write headers
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(len(headers))
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Write data rows
	rowNum := 2
	for idx, kel := range kelurahanList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.ProKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.DapilNama)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelKode)
		colNum++

		// TPS (count) and DPT
		jmlTPS := 0
		jmlDPT := 0
		if tpsInfo, exists := tpsData[kel.KelKode]; exists {
			jmlTPS = tpsInfo.JmlTPS
			jmlDPT = tpsInfo.JmlDPT
		}

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, jmlTPS)
		colNum++

		// KODE TPS (empty for perdesa)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, "")
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, jmlDPT)
		colNum++

		// Party votes
		totalSuara := 0
		for _, partai := range partaiList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)

			kelPartaiData, kelExists := partaiData[kel.KelKode]
			if kelExists {
				if data, partaiExists := kelPartaiData[partai.NomorUrut]; partaiExists {
					f.SetCellValue(sheetName, cell, data.JmlSuaraTotal)
					totalSuara += data.JmlSuaraTotal
				} else {
					f.SetCellValue(sheetName, cell, "-")
				}
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if _, kelExists := partaiData[kel.KelKode]; kelExists {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	lastRow := rowNum - 1
	f.SetCellStyle(sheetName, "A2", lastCol+fmt.Sprintf("%d", lastRow), dataStyle)

	// Set response headers
	filename := fmt.Sprintf("DPR_RI_Partai_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadProvinsiDPRRIPartaiTPS handles Excel download for DPR RI party data per TPS
func (h *DPRDownloadHandler) DownloadProvinsiDPRRIPartaiTPS(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all TPS in this province
	tpsQuery := `
		SELECT
			t.tps_id, t.tps_kode, t.tps_nama, t.kel_kode, t.kec_kode, t.kab_kode,
			t.dapil_kode, t.pro_kode, t.total_dpt,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdpr_wil_tps t
		JOIN pdpr_wil_kel k ON t.kel_kode = k.kel_kode
		JOIN pdpr_wil_kec kec ON t.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON t.kab_kode = kab.kab_kode
		JOIN pdpr_wil_dapil d ON t.dapil_id = d.dapil_id
		WHERE t.pro_kode = ?
		ORDER BY d.dapil_nama, kab.kab_nama, kec.kec_nama, k.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	type TPSInfo struct {
		TPSID     string
		TPSKode   string
		TPSNama   string
		KelKode   string
		KecKode   string
		KabKode   string
		DapilKode string
		ProKode   string
		TotalDPT  int
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var tpsList []TPSInfo
	for tpsRows.Next() {
		var t TPSInfo
		if err := tpsRows.Scan(&t.TPSID, &t.TPSKode, &t.TPSNama, &t.KelKode, &t.KecKode, &t.KabKode,
			&t.DapilKode, &t.ProKode, &t.TotalDPT, &t.KelNama, &t.KecNama, &t.KabNama, &t.DapilNama); err != nil {
			continue
		}
		tpsList = append(tpsList, t)
	}

	// Get vote data from hr_dpr_ri_kel for this province
	voteDataQuery := `SELECT kel_kode, tbl, dapil_id FROM hr_dpr_ri_kel WHERE pro_kode = ?`
	voteRows, err := h.db.Query(voteDataQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Parse TPS-level data from tbl field
	// tbl contains: {"tps_kode": {"caleg_id": votes, ...}, ...}
	tpsVoteData := make(map[string]map[string]int) // tps_kode -> caleg_id -> votes
	kelDapilMap := make(map[string]string)         // kel_kode -> dapil_id

	for voteRows.Next() {
		var kelKode, tblJSON, dapilID string
		if err := voteRows.Scan(&kelKode, &tblJSON, &dapilID); err != nil {
			continue
		}

		kelDapilMap[kelKode] = dapilID

		// Parse TPS data: {"tps_kode": {"caleg_id": votes, ...}, ...}
		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		for tpsKode, calegVotes := range tpsDataMap {
			if _, exists := tpsVoteData[tpsKode]; !exists {
				tpsVoteData[tpsKode] = make(map[string]int)
			}

			for calegID, voteVal := range calegVotes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				tpsVoteData[tpsKode][calegID] = votes
			}
		}
	}

	// Get caleg to party mapping for first dapil (to get party list)
	var firstDapilID string
	for _, dapilID := range kelDapilMap {
		firstDapilID = dapilID
		break
	}

	// Get unique parties
	partaiQuery := `
		SELECT DISTINCT p.id, p.nama, p.partai_singkat, p.nomor_urut
		FROM partai p
		INNER JOIN dpr_ri_caleg c ON p.id = c.partai_id
		WHERE c.dapil_id = ?
		ORDER BY p.nomor_urut
	`
	partaiRows, err := h.db.Query(partaiQuery, firstDapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying party data")
	}
	defer partaiRows.Close()

	type Partai struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}

	var partaiList []Partai
	for partaiRows.Next() {
		var p Partai
		if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
			continue
		}
		partaiList = append(partaiList, p)
	}

	// Get caleg ID to party mapping for all dapils
	calegPartyQuery := `
		SELECT c.id, c.partai_id
		FROM dpr_ri_caleg c
		WHERE c.dapil_id IN (SELECT DISTINCT dapil_id FROM pdpr_wil_kel WHERE pro_kode = ?)
	`
	calegRows, err := h.db.Query(calegPartyQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying caleg data")
	}
	defer calegRows.Close()

	calegPartyMap := make(map[string]int) // caleg_id -> party_id
	for calegRows.Next() {
		var calegID string
		var partaiID int
		if err := calegRows.Scan(&calegID, &partaiID); err != nil {
			continue
		}
		calegPartyMap[calegID] = partaiID
	}

	// Aggregate votes by TPS and party
	tpsPartaiVotes := make(map[string]map[int]int) // tps_kode -> partai_id -> total_votes
	for tpsKode, calegVotes := range tpsVoteData {
		if _, exists := tpsPartaiVotes[tpsKode]; !exists {
			tpsPartaiVotes[tpsKode] = make(map[int]int)
		}

		for calegID, votes := range calegVotes {
			if partaiID, ok := calegPartyMap[calegID]; ok {
				tpsPartaiVotes[tpsKode][partaiID] += votes
			}
		}
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Partai Per TPS"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Set headers
	headers := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Add party columns
	for _, partai := range partaiList {
		headers = append(headers, partai.PartaiSingkat)
	}

	// Add TOTAL column
	headers = append(headers, "TOTAL")

	// Write headers
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(len(headers))
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Write data rows
	rowNum := 2
	for idx, tps := range tpsList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.ProKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilNama)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelKode)
		colNum++

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSNama)
		colNum++

		// KODE TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSKode)
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TotalDPT)
		colNum++

		// Party votes
		totalSuara := 0
		tpsVotes := tpsPartaiVotes[tps.TPSKode]
		for _, partai := range partaiList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			votes := tpsVotes[partai.ID]
			if votes > 0 || len(tpsVotes) > 0 {
				f.SetCellValue(sheetName, cell, votes)
				totalSuara += votes
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if len(tpsVotes) > 0 {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	lastRow := rowNum - 1
	f.SetCellStyle(sheetName, "A2", lastCol+fmt.Sprintf("%d", lastRow), dataStyle)

	// Set response headers
	filename := fmt.Sprintf("DPR_RI_Partai_TPS_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadProvinsiDPRRICalegTPS handles Excel download for DPR RI caleg data per TPS
func (h *DPRDownloadHandler) DownloadProvinsiDPRRICalegTPS(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all TPS in this province
	tpsQuery := `
		SELECT
			t.tps_id, t.tps_kode, t.tps_nama, t.kel_kode, t.kec_kode, t.kab_kode,
			t.dapil_kode, t.pro_kode, t.total_dpt, t.dapil_id,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdpr_wil_tps t
		JOIN pdpr_wil_kel k ON t.kel_kode = k.kel_kode
		JOIN pdpr_wil_kec kec ON t.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON t.kab_kode = kab.kab_kode
		JOIN pdpr_wil_dapil d ON t.dapil_id = d.dapil_id
		WHERE t.pro_kode = ?
		ORDER BY d.dapil_nama, kab.kab_nama, kec.kec_nama, k.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	type TPSInfo struct {
		TPSID     string
		TPSKode   string
		TPSNama   string
		KelKode   string
		KecKode   string
		KabKode   string
		DapilKode string
		ProKode   string
		TotalDPT  int
		DapilID   string
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var tpsList []TPSInfo
	dapilIDs := make(map[string]bool)
	for tpsRows.Next() {
		var t TPSInfo
		if err := tpsRows.Scan(&t.TPSID, &t.TPSKode, &t.TPSNama, &t.KelKode, &t.KecKode, &t.KabKode,
			&t.DapilKode, &t.ProKode, &t.TotalDPT, &t.DapilID, &t.KelNama, &t.KecNama, &t.KabNama, &t.DapilNama); err != nil {
			continue
		}
		tpsList = append(tpsList, t)
		dapilIDs[t.DapilID] = true
	}

	// Get vote data from hr_dpr_ri_kel for this province
	voteDataQuery := `SELECT kel_kode, tbl FROM hr_dpr_ri_kel WHERE pro_kode = ?`
	voteRows, err := h.db.Query(voteDataQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Parse TPS-level data from tbl field
	// tbl contains: {"tps_kode": {"caleg_id": votes, ...}, ...}
	tpsVoteData := make(map[string]map[string]int) // tps_kode -> caleg_id -> votes

	for voteRows.Next() {
		var kelKode, tblJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON); err != nil {
			continue
		}

		// Parse TPS data: {"tps_kode": {"caleg_id": votes, ...}, ...}
		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		for tpsKode, calegVotes := range tpsDataMap {
			if _, exists := tpsVoteData[tpsKode]; !exists {
				tpsVoteData[tpsKode] = make(map[string]int)
			}

			for calegID, voteVal := range calegVotes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				tpsVoteData[tpsKode][calegID] = votes
			}
		}
	}

	// Get all candidates for all dapils in this province
	candidateQuery := `
		SELECT c.id, c.nama, p.nama as nama_partai, p.partai_singkat, c.nomor_urut, c.partai_id, c.dapil_id
		FROM dpr_ri_caleg c
		LEFT JOIN partai p ON c.partai_id = p.id
		WHERE c.dapil_id IN (SELECT DISTINCT dapil_id FROM pdpr_wil_kel WHERE pro_kode = ?)
		ORDER BY c.dapil_id, p.nomor_urut, c.nomor_urut
	`

	candRows, err := h.db.Query(candidateQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying candidate data")
	}
	defer candRows.Close()

	type Candidate struct {
		ID            string
		Nama          string
		Partai        string
		PartaiSingkat string
		NomorUrut     int
		PartaiID      int
		DapilID       string
	}

	var allCandidates []Candidate
	candidatesByDapil := make(map[string][]Candidate)
	for candRows.Next() {
		var c Candidate
		if err := candRows.Scan(&c.ID, &c.Nama, &c.Partai, &c.PartaiSingkat, &c.NomorUrut, &c.PartaiID, &c.DapilID); err != nil {
			continue
		}
		allCandidates = append(allCandidates, c)
		candidatesByDapil[c.DapilID] = append(candidatesByDapil[c.DapilID], c)
	}

	// Get unique parties
	partaiQuery := `
		SELECT DISTINCT p.id, p.nama, p.partai_singkat, p.nomor_urut
		FROM partai p
		INNER JOIN dpr_ri_caleg c ON p.id = c.partai_id
		WHERE c.dapil_id IN (SELECT DISTINCT dapil_id FROM pdpr_wil_kel WHERE pro_kode = ?)
		ORDER BY p.nomor_urut
	`
	partaiRows, err := h.db.Query(partaiQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying party data")
	}
	defer partaiRows.Close()

	type Partai struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}

	var partaiList []Partai
	for partaiRows.Next() {
		var p Partai
		if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
			continue
		}
		partaiList = append(partaiList, p)
	}

	// Create Excel file
	f := excelize.NewFile()
	f.DeleteSheet("Sheet1")

	// Fixed columns
	fixedCols := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Get unique dapil IDs and names
	type DapilInfo struct {
		ID   string
		Nama string
	}
	dapilMap := make(map[string]DapilInfo)
	for _, tps := range tpsList {
		if _, exists := dapilMap[tps.DapilID]; !exists {
			dapilMap[tps.DapilID] = DapilInfo{ID: tps.DapilID, Nama: tps.DapilNama}
		}
	}

	// Convert map to sorted slice
	var dapilList []DapilInfo
	for _, dapil := range dapilMap {
		dapilList = append(dapilList, dapil)
	}

	// If only 1 dapil or want all in one sheet
	createMultipleSheets := len(dapilList) > 1

	// Process each dapil as a separate sheet (or single sheet if only 1 dapil)
	sheetIndex := 0
	for _, dapil := range dapilList {
		var sheetName string
		var tpsForSheet []TPSInfo
		var candidatesForSheet []Candidate

		if createMultipleSheets {
			// Multiple sheets: one per dapil
			sheetName = dapil.Nama
			// Filter TPS for this dapil
			for _, tps := range tpsList {
				if tps.DapilID == dapil.ID {
					tpsForSheet = append(tpsForSheet, tps)
				}
			}
			// Get candidates for this dapil
			candidatesForSheet = candidatesByDapil[dapil.ID]
		} else {
			// Single sheet: all data
			sheetName = "Data Suara Caleg Per TPS"
			tpsForSheet = tpsList
			candidatesForSheet = allCandidates
		}

		// Create sheet
		if sheetIndex == 0 {
			index, err := f.NewSheet(sheetName)
			if err != nil {
				return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
			}
			f.SetActiveSheet(index)
		} else {
			f.NewSheet(sheetName)
		}

		// Calculate total columns: fixed + candidates + total
		totalCols := len(fixedCols) + len(candidatesForSheet) + 1

		// Write headers
		colNum := 1
		for _, col := range fixedCols {
			cell, _ := excelize.CoordinatesToCellName(colNum, 1)
			f.SetCellValue(sheetName, cell, col)
			colNum++
		}

		// Add candidate columns
		for _, cand := range candidatesForSheet {
			cell, _ := excelize.CoordinatesToCellName(colNum, 1)
			headerText := fmt.Sprintf("%s\n%d\n%s", cand.PartaiSingkat, cand.NomorUrut, cand.Nama)
			f.SetCellValue(sheetName, cell, headerText)
			colNum++
		}

		// TOTAL column
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, "TOTAL")

		// Style headers
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
			Border: []excelize.Border{
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
				{Type: "right", Color: "000000", Style: 1},
			},
		})
		lastCol, _ := excelize.ColumnNumberToName(totalCols)
		f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

		// Set column widths
		f.SetColWidth(sheetName, "A", "A", 5)
		f.SetColWidth(sheetName, "B", "B", 20)
		f.SetColWidth(sheetName, "C", "C", 12)
		f.SetColWidth(sheetName, "D", "D", 25)
		f.SetRowHeight(sheetName, 1, 40)

		// Write data rows
		rowNum := 2
		for idx, tps := range tpsForSheet {
			colNum := 1

			// NO
			cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, idx+1)
			colNum++

			// PROVINSI
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, proName)
			colNum++

			// KODE PROV
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.ProKode)
			colNum++

			// DAPIL
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.DapilNama)
			colNum++

			// KODE DAPIL
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.DapilKode)
			colNum++

			// KAB/KOTA
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.KabNama)
			colNum++

			// KODE KAB
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.KabKode)
			colNum++

			// KECAMATAN
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.KecNama)
			colNum++

			// KODE KEC
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.KecKode)
			colNum++

			// KELURAHAN/DESA
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.KelNama)
			colNum++

			// KODE DESA
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.KelKode)
			colNum++

			// TPS
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.TPSNama)
			colNum++

			// KODE TPS
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.TPSKode)
			colNum++

			// DPT
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, tps.TotalDPT)
			colNum++

			// Candidate votes
			totalSuara := 0
			tpsVotes := tpsVoteData[tps.TPSKode]
			for _, cand := range candidatesForSheet {
				cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
				votes := 0
				if tpsVotes != nil {
					if v, ok := tpsVotes[cand.ID]; ok {
						votes = v
					}
				}
				if len(tpsVotes) > 0 {
					f.SetCellValue(sheetName, cell, votes)
					totalSuara += votes
				} else {
					f.SetCellValue(sheetName, cell, "-")
				}
				colNum++
			}

			// TOTAL column
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			if len(tpsVotes) > 0 {
				f.SetCellValue(sheetName, cell, totalSuara)
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}

			rowNum++
		}

		// Apply borders to all cells
		dataStyle, _ := f.NewStyle(&excelize.Style{
			Border: []excelize.Border{
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
				{Type: "right", Color: "000000", Style: 1},
			},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		lastRow := rowNum - 1
		if lastRow >= 2 {
			f.SetCellStyle(sheetName, "A2", lastCol+fmt.Sprintf("%d", lastRow), dataStyle)
		}

		sheetIndex++

		// If single sheet mode, break after first iteration
		if !createMultipleSheets {
			break
		}
	}

	// Set response headers
	filename := fmt.Sprintf("DPR_RI_Caleg_TPS_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadProvinsiDPRRICaleg handles Excel download for all DPR RI caleg data in a province
func (h *DPRDownloadHandler) DownloadProvinsiDPRRICaleg(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all kelurahan in this province
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdpr_wil_kel k
		JOIN pdpr_wil_kec kec ON k.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON kec.kab_kode = kab.kab_kode
		JOIN pdpr_wil_dapil d ON k.dapil_id = d.dapil_id
		WHERE k.pro_kode = ?
		ORDER BY d.dapil_nama, kab.kab_nama, kec.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelurahanQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data")
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var kelurahanList []KelurahanInfo
	dapilMap := make(map[string]struct {
		ID   string
		Name string
	})
	kelurahanByDapil := make(map[string][]KelurahanInfo)

	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama, &k.DapilNama); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
		dapilMap[k.DapilID] = struct {
			ID   string
			Name string
		}{k.DapilID, k.DapilNama}
		kelurahanByDapil[k.DapilID] = append(kelurahanByDapil[k.DapilID], k)
	}

	if len(dapilMap) == 0 {
		return c.String(http.StatusNotFound, "No dapil found for this province")
	}

	// Get TPS count and DPT sum per kelurahan
	tpsQuery := `SELECT kel_kode, COUNT(*) as jml_tps, SUM(COALESCE(total_dpt, 0)) as jml_dpt FROM pdpr_wil_tps WHERE pro_kode = ? GROUP BY kel_kode`

	tpsRows, err := h.db.Query(tpsQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS, jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err != nil {
			continue
		}
		tpsData[kelKode] = struct {
			JmlTPS int
			JmlDPT int
		}{jmlTPS, jmlDPT}
	}

	// Get vote data from hr_dpr_ri_kel
	voteDataQuery := `SELECT kel_kode, tbl, chart FROM hr_dpr_ri_kel WHERE pro_kode = ?`

	voteRows, err := h.db.Query(voteDataQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	chartData := make(map[string]map[string]int)
	partaiData := make(map[string]map[int]struct {
		JmlSuaraTotal  int
		JmlSuaraPartai int
	})

	for voteRows.Next() {
		var kelKode, tblJSON, chartJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON, &chartJSON); err != nil {
			continue
		}

		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		if _, exists := chartData[kelKode]; !exists {
			chartData[kelKode] = make(map[string]int)
		}

		for _, votes := range tpsDataMap {
			for calegID, voteVal := range votes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				chartData[kelKode][calegID] += votes
			}
		}

		var partaiDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(chartJSON), &partaiDataMap); err != nil {
			continue
		}

		if _, exists := partaiData[kelKode]; !exists {
			partaiData[kelKode] = make(map[int]struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			})
		}

		for nomorUrutStr, data := range partaiDataMap {
			nomorUrut, _ := strconv.Atoi(nomorUrutStr)
			jmlSuaraTotal := 0
			jmlSuaraPartai := 0

			if val, ok := data["jml_suara_total"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraTotal = int(v)
				case int:
					jmlSuaraTotal = v
				case string:
					jmlSuaraTotal, _ = strconv.Atoi(v)
				}
			}

			if val, ok := data["jml_suara_partai"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraPartai = int(v)
				case int:
					jmlSuaraPartai = v
				case string:
					jmlSuaraPartai, _ = strconv.Atoi(v)
				}
			}

			partaiData[kelKode][nomorUrut] = struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			}{jmlSuaraTotal, jmlSuaraPartai}
		}
	}

	// Create Excel file
	f := excelize.NewFile()

	type Candidate struct {
		ID            string
		Nama          string
		Partai        string
		PartaiSingkat string
		NomorUrut     int
		PartaiID      int
		DapilID       string
	}

	type Partai struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}

	// Fixed columns
	fixedCols := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Process each dapil
	sheetIndex := 0
	for dapilID, dapilInfo := range dapilMap {
		// Get candidates for this specific dapil
		candidateQuery := `
			SELECT c.id, c.nama, p.nama as nama_partai, p.partai_singkat, c.nomor_urut, c.partai_id, c.dapil_id
			FROM dpr_ri_caleg c
			LEFT JOIN partai p ON c.partai_id = p.id
			WHERE c.dapil_id = ?
			ORDER BY p.nomor_urut, c.nomor_urut
		`

		candRows, err := h.db.Query(candidateQuery, dapilID)
		if err != nil {
			continue
		}

		var candidates []Candidate
		candidatesByPartai := make(map[int][]Candidate)
		for candRows.Next() {
			var c Candidate
			if err := candRows.Scan(&c.ID, &c.Nama, &c.Partai, &c.PartaiSingkat, &c.NomorUrut, &c.PartaiID, &c.DapilID); err != nil {
				continue
			}
			candidates = append(candidates, c)
			candidatesByPartai[c.PartaiID] = append(candidatesByPartai[c.PartaiID], c)
		}
		candRows.Close()

		// Get unique parties for this dapil
		partaiQuery := `
			SELECT DISTINCT p.id, p.nama, p.partai_singkat, p.nomor_urut
			FROM partai p
			INNER JOIN dpr_ri_caleg c ON p.id = c.partai_id
			WHERE c.dapil_id = ?
			ORDER BY p.nomor_urut
		`
		partaiRows, err := h.db.Query(partaiQuery, dapilID)
		if err != nil {
			continue
		}

		var partaiList []Partai
		for partaiRows.Next() {
			var p Partai
			if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
				continue
			}
			partaiList = append(partaiList, p)
		}
		partaiRows.Close()

		// Create sheet for this dapil
		sheetName := dapilInfo.Name
		if sheetIndex == 0 {
			index, err := f.NewSheet(sheetName)
			if err != nil {
				continue
			}
			f.SetActiveSheet(index)
			// Delete Sheet1 after creating first sheet
			f.DeleteSheet("Sheet1")
		} else {
			_, err := f.NewSheet(sheetName)
			if err != nil {
				continue
			}
		}
		sheetIndex++

		// Calculate total columns
		totalCols := len(fixedCols) + (len(partaiList) * 2) + len(candidates) + 1

		// Create 3-row headers
		colNum := 1

		f.SetCellValue(sheetName, "A1", "partai")
		f.SetCellValue(sheetName, "A2", "nomor urut caleg")

		for idx, col := range fixedCols {
			cell3, _ := excelize.CoordinatesToCellName(idx+1, 3)
			f.SetCellValue(sheetName, cell3, col)
		}

		fixedColEnd, _ := excelize.ColumnNumberToName(len(fixedCols))
		f.MergeCell(sheetName, "A1", fixedColEnd+"1")
		f.MergeCell(sheetName, "A2", fixedColEnd+"2")

		colNum = len(fixedCols) + 1

		for _, partai := range partaiList {
			startCol := colNum

			cell2, _ := excelize.CoordinatesToCellName(colNum, 2)
			f.SetCellValue(sheetName, cell2, "")
			cell3, _ := excelize.CoordinatesToCellName(colNum, 3)
			f.SetCellValue(sheetName, cell3, fmt.Sprintf("TOTAL_PARTAI_%d", partai.NomorUrut))
			colNum++

			cell2, _ = excelize.CoordinatesToCellName(colNum, 2)
			f.SetCellValue(sheetName, cell2, "")
			cell3, _ = excelize.CoordinatesToCellName(colNum, 3)
			f.SetCellValue(sheetName, cell3, fmt.Sprintf("GAMBAR_PARTAI_%d", partai.NomorUrut))
			colNum++

			if calegs, exists := candidatesByPartai[partai.ID]; exists {
				for _, cand := range calegs {
					cell2, _ := excelize.CoordinatesToCellName(colNum, 2)
					f.SetCellValue(sheetName, cell2, cand.NomorUrut)
					cell3, _ := excelize.CoordinatesToCellName(colNum, 3)
					f.SetCellValue(sheetName, cell3, cand.Nama)
					colNum++
				}
			}

			endCol := colNum - 1
			startCell, _ := excelize.CoordinatesToCellName(startCol, 1)
			endCell, _ := excelize.CoordinatesToCellName(endCol, 1)
			f.SetCellValue(sheetName, startCell, partai.PartaiSingkat)
			if startCol != endCol {
				f.MergeCell(sheetName, startCell, endCell)
			}
		}

		for row := 1; row <= 3; row++ {
			cell, _ := excelize.CoordinatesToCellName(colNum, row)
			f.SetCellValue(sheetName, cell, "TOTAL SUARA")
		}

		headerStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 11},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
			Border: []excelize.Border{
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
				{Type: "right", Color: "000000", Style: 1},
			},
		})
		lastCol, _ := excelize.ColumnNumberToName(totalCols)
		f.SetCellStyle(sheetName, "A1", lastCol+"3", headerStyle)

		// Write data rows for this dapil
		rowNum := 4
		dapilKelList := kelurahanByDapil[dapilID]
		for idx, kel := range dapilKelList {
			colNum := 1

			// Get TPS and DPT data
			jmlTPS := 0
			jmlDPT := 0
			if tpsInfo, exists := tpsData[kel.KelKode]; exists {
				jmlTPS = tpsInfo.JmlTPS
				jmlDPT = tpsInfo.JmlDPT
			}

			// NO
			cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, idx+1)
			colNum++

			// PROVINSI
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, proName)
			colNum++

			// KODE PROV
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.ProKode)
			colNum++

			// DAPIL
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.DapilNama)
			colNum++

			// KODE DAPIL
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.DapilKode)
			colNum++

			// KAB/KOTA
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.KabNama)
			colNum++

			// KODE KAB
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.KabKode)
			colNum++

			// KECAMATAN
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.KecNama)
			colNum++

			// KODE KEC
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.KecKode)
			colNum++

			// KELURAHAN/DESA
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.KelNama)
			colNum++

			// KODE DESA
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, kel.KelKode)
			colNum++

			// TPS (count)
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, jmlTPS)
			colNum++

			// KODE TPS (empty for perdesa)
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, "")
			colNum++

			// DPT
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, jmlDPT)
			colNum++

			kelVotes := chartData[kel.KelKode]

			totalVotes := 0
			if kelPartaiData, exists := partaiData[kel.KelKode]; exists {
				for _, data := range kelPartaiData {
					totalVotes += data.JmlSuaraTotal
				}
			}

			for _, partai := range partaiList {
				cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
				totalPartai := 0
				gambarPartai := 0
				if kelPartaiData, exists := partaiData[kel.KelKode]; exists {
					if data, ok := kelPartaiData[partai.NomorUrut]; ok {
						totalPartai = data.JmlSuaraTotal
						gambarPartai = data.JmlSuaraPartai
					}
				}
				f.SetCellValue(sheetName, cell, totalPartai)
				colNum++

				cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
				f.SetCellValue(sheetName, cell, gambarPartai)
				colNum++

				if calegs, exists := candidatesByPartai[partai.ID]; exists {
					for _, cand := range calegs {
						cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
						votes := 0
						if kelVotes != nil {
							if v, ok := kelVotes[cand.ID]; ok {
								votes = v
							}
						}
						f.SetCellValue(sheetName, cell, votes)
						colNum++
					}
				}
			}

			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			f.SetCellValue(sheetName, cell, totalVotes)

			rowNum++
		}

		dataStyle, _ := f.NewStyle(&excelize.Style{
			Border: []excelize.Border{
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
				{Type: "right", Color: "000000", Style: 1},
			},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		lastRow := rowNum - 1
		if lastRow >= 4 {
			f.SetCellStyle(sheetName, "A4", lastCol+fmt.Sprintf("%d", lastRow), dataStyle)
		}
	}

	filename := fmt.Sprintf("DPR_RI_Caleg_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadKabupatenDPRRIPartaiTPS handles Excel download for DPR RI party data per TPS in a kabupaten
func (h *DPRDownloadHandler) DownloadKabupatenDPRRIPartaiTPS(c echo.Context) error {
	kabCode := c.Param("code")

	// Get kabupaten name and province code
	var kabName, proCode string
	err := h.db.QueryRow("SELECT kab_nama, pro_kode FROM pdpr_wil_kab WHERE kab_kode = ?", kabCode).Scan(&kabName, &proCode)
	if err != nil {
		return c.String(http.StatusNotFound, "Kabupaten tidak ditemukan")
	}

	// Get province name
	var proName string
	err = h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = "Unknown"
	}

	// Get all TPS in this kabupaten
	tpsQuery := `
		SELECT
			t.tps_kode, t.tps_nama, t.kel_kode, t.kec_kode, t.kab_kode,
			t.dapil_kode, t.pro_kode, t.total_dpt,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdpr_wil_tps t
		JOIN pdpr_wil_kel k ON t.kel_kode = k.kel_kode
		JOIN pdpr_wil_kec kec ON t.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON t.kab_kode = kab.kab_kode
		JOIN pdpr_wil_dapil d ON t.dapil_id = d.dapil_id
		WHERE t.kab_kode = ?
		ORDER BY d.dapil_nama, kec.kec_nama, k.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, kabCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	type TPSInfo struct {
		TPSKode   string
		TPSNama   string
		KelKode   string
		KecKode   string
		KabKode   string
		DapilKode string
		ProKode   string
		TotalDPT  int
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var tpsList []TPSInfo
	for tpsRows.Next() {
		var t TPSInfo
		if err := tpsRows.Scan(&t.TPSKode, &t.TPSNama, &t.KelKode, &t.KecKode, &t.KabKode,
			&t.DapilKode, &t.ProKode, &t.TotalDPT, &t.KelNama, &t.KecNama, &t.KabNama, &t.DapilNama); err != nil {
			continue
		}
		tpsList = append(tpsList, t)
	}

	// Get vote data from hr_dpr_ri_kel for this kabupaten
	voteDataQuery := `SELECT kel_kode, tbl FROM hr_dpr_ri_kel WHERE kab_kode = ?`
	voteRows, err := h.db.Query(voteDataQuery, kabCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Parse TPS-level data from tbl field
	tpsVoteData := make(map[string]map[string]int) // tps_kode -> caleg_id -> votes

	for voteRows.Next() {
		var kelKode, tblJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON); err != nil {
			continue
		}

		// Parse TPS data
		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		for tpsKode, calegVotes := range tpsDataMap {
			if _, exists := tpsVoteData[tpsKode]; !exists {
				tpsVoteData[tpsKode] = make(map[string]int)
			}

			for calegID, voteVal := range calegVotes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				tpsVoteData[tpsKode][calegID] = votes
			}
		}
	}

	// Get caleg to party mapping
	calegToParty := make(map[string]int) // caleg_id -> party_id
	calegRows, err := h.db.Query(`
		SELECT c.id, c.partai_id
		FROM dpr_ri_caleg c
		WHERE c.dapil_id IN (SELECT DISTINCT dapil_id FROM pdpr_wil_kel WHERE kab_kode = ?)
	`, kabCode)
	if err == nil {
		defer calegRows.Close()
		for calegRows.Next() {
			var calegID, partyID int
			calegRows.Scan(&calegID, &partyID)
			calegToParty[strconv.Itoa(calegID)] = partyID
		}
	}

	// Get all parties
	type Partai struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}
	var partaiList []Partai
	partaiRows, err := h.db.Query(`
		SELECT id, nama, partai_singkat, nomor_urut
		FROM partai
		WHERE id BETWEEN 1 AND 24
		ORDER BY nomor_urut
	`)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying party data")
	}
	defer partaiRows.Close()

	for partaiRows.Next() {
		var p Partai
		if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
			continue
		}
		partaiList = append(partaiList, p)
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Partai Per TPS"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Fixed columns
	fixedCols := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Calculate total columns
	totalCols := len(fixedCols) + len(partaiList) + 1

	// Write headers
	colNum := 1
	for _, col := range fixedCols {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, col)
		colNum++
	}

	// Add party columns
	for _, partai := range partaiList {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, partai.PartaiSingkat)
		colNum++
	}

	// TOTAL column
	cell, _ := excelize.CoordinatesToCellName(colNum, 1)
	f.SetCellValue(sheetName, cell, "TOTAL")

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(totalCols)
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 12)
	f.SetColWidth(sheetName, "D", "D", 25)
	f.SetRowHeight(sheetName, 1, 30)

	// Write data rows
	rowNum := 2
	for idx, tps := range tpsList {
		colNum := 1

		// Fixed columns
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.ProKode)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilNama)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilKode)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabNama)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabKode)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecNama)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecKode)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelNama)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelKode)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSNama)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSKode)
		colNum++

		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TotalDPT)
		colNum++

		// Party votes - aggregate by party
		tpsVotes := tpsVoteData[tps.TPSKode]
		partyVotes := make(map[int]int)
		for calegID, votes := range tpsVotes {
			if partyID, ok := calegToParty[calegID]; ok {
				partyVotes[partyID] += votes
			}
		}

		totalSuara := 0
		for _, partai := range partaiList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			votes := partyVotes[partai.ID]
			if len(tpsVotes) > 0 {
				f.SetCellValue(sheetName, cell, votes)
				totalSuara += votes
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if len(tpsVotes) > 0 {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if len(tpsList) > 0 {
		lastRow := len(tpsList) + 1
		f.SetCellStyle(sheetName, "A2", lastCol+strconv.Itoa(lastRow), dataStyle)
	}

	filename := fmt.Sprintf("DPR_RI_Partai_TPS_%s.xlsx", kabName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadKabupatenDPRRICalegTPS handles Excel download for DPR RI caleg data per TPS in a kabupaten
func (h *DPRDownloadHandler) DownloadKabupatenDPRRICalegTPS(c echo.Context) error {
	kabCode := c.Param("code")

	// Get kabupaten name and province code
	var kabName, proCode string
	err := h.db.QueryRow("SELECT kab_nama, pro_kode FROM pdpr_wil_kab WHERE kab_kode = ?", kabCode).Scan(&kabName, &proCode)
	if err != nil {
		return c.String(http.StatusNotFound, "Kabupaten tidak ditemukan")
	}

	// Get province name
	var proName string
	err = h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = "Unknown"
	}

	// Get all TPS in this kabupaten
	tpsQuery := `
		SELECT
			t.tps_id, t.tps_kode, t.tps_nama, t.kel_kode, t.kec_kode, t.kab_kode,
			t.dapil_kode, t.pro_kode, t.total_dpt, t.dapil_id,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdpr_wil_tps t
		JOIN pdpr_wil_kel k ON t.kel_kode = k.kel_kode
		JOIN pdpr_wil_kec kec ON t.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON t.kab_kode = kab.kab_kode
		JOIN pdpr_wil_dapil d ON t.dapil_id = d.dapil_id
		WHERE t.kab_kode = ?
		ORDER BY d.dapil_nama, kec.kec_nama, k.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, kabCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	type TPSInfo struct {
		TPSID     string
		TPSKode   string
		TPSNama   string
		KelKode   string
		KecKode   string
		KabKode   string
		DapilKode string
		ProKode   string
		TotalDPT  int
		DapilID   string
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var tpsList []TPSInfo
	dapilIDs := make(map[string]bool)
	for tpsRows.Next() {
		var t TPSInfo
		if err := tpsRows.Scan(&t.TPSID, &t.TPSKode, &t.TPSNama, &t.KelKode, &t.KecKode, &t.KabKode,
			&t.DapilKode, &t.ProKode, &t.TotalDPT, &t.DapilID, &t.KelNama, &t.KecNama, &t.KabNama, &t.DapilNama); err != nil {
			continue
		}
		tpsList = append(tpsList, t)
		dapilIDs[t.DapilID] = true
	}

	// Get vote data from hr_dpr_ri_kel for this kabupaten
	voteDataQuery := `SELECT kel_kode, tbl FROM hr_dpr_ri_kel WHERE kab_kode = ?`
	voteRows, err := h.db.Query(voteDataQuery, kabCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Parse TPS-level data from tbl field
	tpsVoteData := make(map[string]map[string]int) // tps_kode -> caleg_id -> votes

	for voteRows.Next() {
		var kelKode, tblJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON); err != nil {
			continue
		}

		// Parse TPS data: {"tps_kode": {"caleg_id": votes, ...}, ...}
		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		for tpsKode, calegVotes := range tpsDataMap {
			if _, exists := tpsVoteData[tpsKode]; !exists {
				tpsVoteData[tpsKode] = make(map[string]int)
			}

			for calegID, voteVal := range calegVotes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				tpsVoteData[tpsKode][calegID] = votes
			}
		}
	}

	// Get all candidates for all dapils in this kabupaten
	candidateQuery := `
		SELECT c.id, c.nama, p.nama as nama_partai, p.partai_singkat, c.nomor_urut, c.partai_id, c.dapil_id
		FROM dpr_ri_caleg c
		LEFT JOIN partai p ON c.partai_id = p.id
		WHERE c.dapil_id IN (SELECT DISTINCT dapil_id FROM pdpr_wil_kel WHERE kab_kode = ?)
		ORDER BY c.dapil_id, p.nomor_urut, c.nomor_urut
	`

	candRows, err := h.db.Query(candidateQuery, kabCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying candidate data")
	}
	defer candRows.Close()

	type Candidate struct {
		ID            string
		Nama          string
		Partai        string
		PartaiSingkat string
		NomorUrut     int
		PartaiID      int
		DapilID       string
	}

	var allCandidates []Candidate
	for candRows.Next() {
		var c Candidate
		if err := candRows.Scan(&c.ID, &c.Nama, &c.Partai, &c.PartaiSingkat, &c.NomorUrut, &c.PartaiID, &c.DapilID); err != nil {
			continue
		}
		allCandidates = append(allCandidates, c)
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Caleg Per TPS"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Fixed columns
	fixedCols := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Calculate total columns: fixed + candidates + total
	totalCols := len(fixedCols) + len(allCandidates) + 1

	// Write headers
	colNum := 1
	for _, col := range fixedCols {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, col)
		colNum++
	}

	// Add candidate columns with new format: Partai\nNomor\nNama
	for _, cand := range allCandidates {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		headerText := fmt.Sprintf("%s\n%d\n%s", cand.PartaiSingkat, cand.NomorUrut, cand.Nama)
		f.SetCellValue(sheetName, cell, headerText)
		colNum++
	}

	// TOTAL column
	cell, _ := excelize.CoordinatesToCellName(colNum, 1)
	f.SetCellValue(sheetName, cell, "TOTAL")

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(totalCols)
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 12)
	f.SetColWidth(sheetName, "D", "D", 25)
	f.SetRowHeight(sheetName, 1, 40)

	// Write data rows
	rowNum := 2
	for idx, tps := range tpsList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.ProKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilNama)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelKode)
		colNum++

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSNama)
		colNum++

		// KODE TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSKode)
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TotalDPT)
		colNum++

		// Candidate votes
		totalSuara := 0
		tpsVotes := tpsVoteData[tps.TPSKode]
		for _, cand := range allCandidates {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			votes := 0
			if tpsVotes != nil {
				if v, ok := tpsVotes[cand.ID]; ok {
					votes = v
				}
			}
			if len(tpsVotes) > 0 {
				f.SetCellValue(sheetName, cell, votes)
				totalSuara += votes
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if len(tpsVotes) > 0 {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if len(tpsList) > 0 {
		lastRow := len(tpsList) + 1
		f.SetCellStyle(sheetName, "A2", lastCol+strconv.Itoa(lastRow), dataStyle)
	}

	filename := fmt.Sprintf("DPR_RI_Caleg_TPS_%s.xlsx", kabName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadProvinsiPilpres handles Excel download for PILPRES (Presidential Election) data per kelurahan in a province
func (h *DPRDownloadHandler) DownloadProvinsiPilpres(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM ppwp_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all kelurahan in this province with vote data
	// Optimized query: join only necessary tables
	kelQuery := `
		SELECT
			k.kel_kode, k.kel_nama, k.kec_nama, k.kab_nama,
			COALESCE(hr.chart, '{}') as chart
		FROM ppwp_wil_kel k
		LEFT JOIN hr_pilpres_kel hr ON k.kel_kode = hr.kel_kode
		WHERE k.pro_kode = ?
		ORDER BY k.kab_nama, k.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data")
	}
	defer kelRows.Close()

	type KelurahanData struct {
		KelKode  string
		KelNama  string
		KecNama  string
		KabNama  string
		ChartJSON string
	}

	var kelurahanList []KelurahanData
	for kelRows.Next() {
		var k KelurahanData
		if err := kelRows.Scan(&k.KelKode, &k.KelNama, &k.KecNama, &k.KabNama, &k.ChartJSON); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
	}

	// Get presidential candidates
	type Paslon struct {
		ID          int
		NamaSingkat string
		NomorUrut   string
	}

	var paslonList []Paslon
	paslonRows, err := h.db.Query(`
		SELECT id, nama_singkat, nomor_urut
		FROM ppwp
		ORDER BY nomor_urut
	`)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying candidate data")
	}
	defer paslonRows.Close()

	for paslonRows.Next() {
		var p Paslon
		if err := paslonRows.Scan(&p.ID, &p.NamaSingkat, &p.NomorUrut); err != nil {
			continue
		}
		paslonList = append(paslonList, p)
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data PILPRES Per Kelurahan"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Fixed columns
	fixedCols := []string{
		"NO", "PROVINSI", "KAB/KOTA", "KECAMATAN", "KELURAHAN/DESA", "KODE DESA",
	}

	// Calculate total columns
	totalCols := len(fixedCols) + len(paslonList) + 1

	// Write headers
	colNum := 1
	for _, col := range fixedCols {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, col)
		colNum++
	}

	// Add paslon columns
	for _, paslon := range paslonList {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		headerText := fmt.Sprintf("%s\n%s", paslon.NomorUrut, paslon.NamaSingkat)
		f.SetCellValue(sheetName, cell, headerText)
		colNum++
	}

	// TOTAL column
	cell, _ := excelize.CoordinatesToCellName(colNum, 1)
	f.SetCellValue(sheetName, cell, "TOTAL")

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(totalCols)
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 20)
	f.SetColWidth(sheetName, "D", "D", 20)
	f.SetColWidth(sheetName, "E", "E", 25)
	f.SetRowHeight(sheetName, 1, 35)

	// Write data rows
	rowNum := 2
	for idx, kel := range kelurahanList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabNama)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecNama)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelKode)
		colNum++

		// Parse chart JSON: {"100025":597,"100026":119,"100027":21}
		paslonVotes := make(map[int]int)
		if kel.ChartJSON != "" && kel.ChartJSON != "{}" {
			var chartData map[string]interface{}
			if err := json.Unmarshal([]byte(kel.ChartJSON), &chartData); err == nil {
				for paslonIDStr, votesVal := range chartData {
					paslonID, _ := strconv.Atoi(paslonIDStr)
					votes := 0
					switch v := votesVal.(type) {
					case float64:
						votes = int(v)
					case int:
						votes = v
					case string:
						votes, _ = strconv.Atoi(v)
					}
					paslonVotes[paslonID] = votes
				}
			}
		}

		// Paslon votes
		totalSuara := 0
		hasData := len(paslonVotes) > 0
		for _, paslon := range paslonList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			votes := paslonVotes[paslon.ID]
			if hasData {
				f.SetCellValue(sheetName, cell, votes)
				totalSuara += votes
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if hasData {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if len(kelurahanList) > 0 {
		lastRow := len(kelurahanList) + 1
		f.SetCellStyle(sheetName, "A2", lastCol+strconv.Itoa(lastRow), dataStyle)
	}

	filename := fmt.Sprintf("PILPRES_Per_Kelurahan_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadProvinsiDPD handles Excel download for DPD (Regional Representatives) data per kelurahan in a province
func (h *DPRDownloadHandler) DownloadProvinsiDPD(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM ppwp_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all kelurahan in this province with vote data
	// Join with pdpr_wil_kel to get dapil info
	kelQuery := `
		SELECT
			ppw.kel_kode, ppw.kel_nama,
			ppw.kec_kode, ppw.kec_nama,
			ppw.kab_kode, ppw.kab_nama,
			COALESCE(pdr.dapil_kode, '') as dapil_kode,
			COALESCE(d.dapil_nama, '') as dapil_nama,
			COALESCE(ppw.tps_kode, '') as tps_kode,
			COALESCE(ppw.tps_nama, '') as tps_nama,
			COALESCE(hr.chart, '{}') as chart,
			COALESCE(hr.administrasi, '{}') as administrasi
		FROM ppwp_wil_kel ppw
		LEFT JOIN pdpr_wil_kel pdr ON ppw.kel_kode = pdr.kel_kode
		LEFT JOIN pdpr_wil_dapil d ON pdr.dapil_id = d.dapil_id AND pdr.kab_kode = '0'
		LEFT JOIN hr_dpd_kel hr ON ppw.kel_kode = hr.kel_kode
		WHERE ppw.pro_kode = ?
		ORDER BY ppw.kab_nama, ppw.kec_nama, ppw.kel_nama
	`

	kelRows, err := h.db.Query(kelQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data")
	}
	defer kelRows.Close()

	type KelurahanData struct {
		KelKode         string
		KelNama         string
		KecKode         string
		KecNama         string
		KabKode         string
		KabNama         string
		DapilKode       string
		DapilNama       string
		TPSKode         string
		TPSNama         string
		ChartJSON       string
		AdministrasiJSON string
	}

	var kelurahanList []KelurahanData
	for kelRows.Next() {
		var k KelurahanData
		if err := kelRows.Scan(&k.KelKode, &k.KelNama, &k.KecKode, &k.KecNama, &k.KabKode, &k.KabNama, &k.DapilKode, &k.DapilNama, &k.TPSKode, &k.TPSNama, &k.ChartJSON, &k.AdministrasiJSON); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
	}

	// Get DPD candidates for this province
	type Caleg struct {
		ID        int
		Nama      string
		NomorUrut int
	}

	var calegList []Caleg
	calegRows, err := h.db.Query(`
		SELECT id, nama, nomor_urut
		FROM dpd_caleg
		WHERE pro_kode = ?
		ORDER BY nomor_urut
	`, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying candidate data")
	}
	defer calegRows.Close()

	for calegRows.Next() {
		var c Caleg
		if err := calegRows.Scan(&c.ID, &c.Nama, &c.NomorUrut); err != nil {
			continue
		}
		calegList = append(calegList, c)
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data DPD Per Kelurahan"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Fixed columns - data per kelurahan
	fixedCols := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Calculate total columns
	totalCols := len(fixedCols) + len(calegList) + 1

	// Write headers
	colNum := 1
	for _, col := range fixedCols {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, col)
		colNum++
	}

	// Add caleg columns
	for _, caleg := range calegList {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		headerText := fmt.Sprintf("%d\n%s", caleg.NomorUrut, caleg.Nama)
		f.SetCellValue(sheetName, cell, headerText)
		colNum++
	}

	// TOTAL column
	cell, _ := excelize.CoordinatesToCellName(colNum, 1)
	f.SetCellValue(sheetName, cell, "TOTAL")

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(totalCols)
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 12)
	f.SetColWidth(sheetName, "D", "D", 20)
	f.SetColWidth(sheetName, "E", "E", 12)
	f.SetColWidth(sheetName, "F", "F", 20)
	f.SetRowHeight(sheetName, 1, 35)

	// Write data rows
	rowNum := 2
	for idx, kel := range kelurahanList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proCode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.DapilNama)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.KelKode)
		colNum++

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.TPSNama)
		colNum++

		// KODE TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kel.TPSKode)
		colNum++

		// Parse administrasi data for DPT
		var dpt int
		if kel.AdministrasiJSON != "" && kel.AdministrasiJSON != "{}" {
			var administrasi map[string]interface{}
			if err := json.Unmarshal([]byte(kel.AdministrasiJSON), &administrasi); err == nil {
				if val, ok := administrasi["pemilih_dpt_j"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						dpt = int(v)
					}
				}
			}
		}

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dpt)
		colNum++

		// Parse chart JSON: {"695637":41,"695638":35,...}
		calegVotes := make(map[int]int)
		if kel.ChartJSON != "" && kel.ChartJSON != "{}" {
			var chartData map[string]interface{}
			if err := json.Unmarshal([]byte(kel.ChartJSON), &chartData); err == nil {
				for calegIDStr, votesVal := range chartData {
					if calegIDStr == "null" {
						continue
					}
					calegID, _ := strconv.Atoi(calegIDStr)
					votes := 0
					switch v := votesVal.(type) {
					case float64:
						votes = int(v)
					case int:
						votes = v
					case string:
						votes, _ = strconv.Atoi(v)
					}
					calegVotes[calegID] = votes
				}
			}
		}

		// Caleg votes
		totalSuara := 0
		hasData := len(calegVotes) > 0
		for _, caleg := range calegList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			votes := calegVotes[caleg.ID]
			if hasData {
				f.SetCellValue(sheetName, cell, votes)
				totalSuara += votes
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if hasData {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if len(kelurahanList) > 0 {
		lastRow := len(kelurahanList) + 1
		f.SetCellStyle(sheetName, "A2", lastCol+strconv.Itoa(lastRow), dataStyle)
	}

	filename := fmt.Sprintf("DPD_Per_Kelurahan_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadProvinsiDPRDProvPartai handles Excel download for DPRD Provinsi party data by province
func (h *DPRDownloadHandler) DownloadProvinsiDPRDProvPartai(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM pdprdp_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Optimized query: Get all kelurahan in this province with JOIN to get names
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdprdp_wil_kel k
		JOIN pdprdp_wil_kec kec ON k.kec_kode = kec.kec_kode AND k.kab_kode = kec.kab_kode
		JOIN pdprdp_wil_kab kab ON k.kab_kode = kab.kab_kode
		JOIN dprd_pro_dapil d ON k.dapil_id = d.dapil_id
		WHERE k.pro_kode = ?
		ORDER BY d.dapil_nama, kab.kab_nama, kec.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelurahanQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data: "+err.Error())
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var kelurahanList []KelurahanInfo
	dapilIDs := make(map[string]bool)
	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama, &k.DapilNama); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
		dapilIDs[k.DapilID] = true
	}

	if len(kelurahanList) == 0 {
		return c.String(http.StatusNotFound, "Tidak ada data kelurahan untuk provinsi ini")
	}

	// Optimized query: Get TPS count and DPT sum per kelurahan
	tpsQuery := `
		SELECT kel_kode, COUNT(*) as jml_tps, COALESCE(SUM(total_dpt), 0) as jml_dpt
		FROM pdpr_wil_tps
		WHERE pro_kode = ?
		GROUP BY kel_kode
	`

	tpsRows, err := h.db.Query(tpsQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS int
		var jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err == nil {
			tpsData[kelKode] = struct {
				JmlTPS int
				JmlDPT int
			}{JmlTPS: jmlTPS, JmlDPT: jmlDPT}
		}
	}

	// Optimized query: Get party vote data from hr_dprd_pro_kel using IN clause
	kelKodes := make([]interface{}, len(kelurahanList))
	for i, k := range kelurahanList {
		kelKodes[i] = k.KelKode
	}

	// Build placeholders for IN clause
	placeholders := ""
	for i := range kelKodes {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}

	voteQuery := fmt.Sprintf(`
		SELECT kel_kode, chart
		FROM hr_dprd_pro_kel
		WHERE pro_kode = ? AND kel_kode IN (%s)
	`, placeholders)

	args := make([]interface{}, len(kelKodes)+1)
	args[0] = proCode
	copy(args[1:], kelKodes)

	voteRows, err := h.db.Query(voteQuery, args...)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data: "+err.Error())
	}
	defer voteRows.Close()

	// Parse party data from chart JSON
	partaiData := make(map[string]map[int]struct {
		JmlSuaraTotal  int
		JmlSuaraPartai int
	})

	for voteRows.Next() {
		var kelKode string
		var chartJSON sql.NullString
		if err := voteRows.Scan(&kelKode, &chartJSON); err != nil {
			continue
		}

		if !chartJSON.Valid || chartJSON.String == "" {
			continue
		}

		var partaiDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(chartJSON.String), &partaiDataMap); err != nil {
			continue
		}

		if _, exists := partaiData[kelKode]; !exists {
			partaiData[kelKode] = make(map[int]struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			})
		}

		for nomorUrutStr, data := range partaiDataMap {
			nomorUrut, _ := strconv.Atoi(nomorUrutStr)
			jmlSuaraTotal := 0
			jmlSuaraPartai := 0

			if val, ok := data["jml_suara_total"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraTotal = int(v)
				case int:
					jmlSuaraTotal = v
				case string:
					jmlSuaraTotal, _ = strconv.Atoi(v)
				}
			}

			if val, ok := data["jml_suara_partai"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraPartai = int(v)
				case int:
					jmlSuaraPartai = v
				case string:
					jmlSuaraPartai, _ = strconv.Atoi(v)
				}
			}

			partaiData[kelKode][nomorUrut] = struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			}{JmlSuaraTotal: jmlSuaraTotal, JmlSuaraPartai: jmlSuaraPartai}
		}
	}

	// Get parties list dynamically from database
	// Only include Aceh local parties if province is Aceh (code = '11')
	var partaiQuery string
	if proCode == "11" {
		partaiQuery = "SELECT nomor_urut, partai_singkat FROM partai ORDER BY nomor_urut"
	} else {
		partaiQuery = "SELECT nomor_urut, partai_singkat FROM partai WHERE is_aceh = 'false' OR is_aceh IS NULL ORDER BY nomor_urut"
	}

	partaiRows, err := h.db.Query(partaiQuery)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying partai data: "+err.Error())
	}
	defer partaiRows.Close()

	type PartaiInfo struct {
		NomorUrut    int
		PartaiSingkat string
	}

	var partaiList []PartaiInfo
	for partaiRows.Next() {
		var p PartaiInfo
		if err := partaiRows.Scan(&p.NomorUrut, &p.PartaiSingkat); err == nil {
			partaiList = append(partaiList, p)
		}
	}

	if len(partaiList) == 0 {
		return c.String(http.StatusInternalServerError, "Tidak ada data partai")
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data DPRD Prov Partai"
	f.SetSheetName("Sheet1", sheetName)

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Write headers
	headers := []string{"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL", "KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC", "KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT"}
	for _, partai := range partaiList {
		headers = append(headers, partai.PartaiSingkat)
	}

	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)   // NO
	f.SetColWidth(sheetName, "B", "B", 25)  // PROVINSI
	f.SetColWidth(sheetName, "C", "C", 12)  // KODE PROV
	f.SetColWidth(sheetName, "D", "D", 20)  // DAPIL
	f.SetColWidth(sheetName, "E", "E", 12)  // KODE DAPIL
	f.SetColWidth(sheetName, "F", "F", 25)  // KAB/KOTA
	f.SetColWidth(sheetName, "G", "G", 12)  // KODE KAB
	f.SetColWidth(sheetName, "H", "H", 25)  // KECAMATAN
	f.SetColWidth(sheetName, "I", "I", 12)  // KODE KEC
	f.SetColWidth(sheetName, "J", "J", 30)  // KELURAHAN/DESA
	f.SetColWidth(sheetName, "K", "K", 12)  // KODE DESA
	f.SetColWidth(sheetName, "L", "L", 10)  // TPS
	f.SetColWidth(sheetName, "M", "M", 12)  // KODE TPS
	f.SetColWidth(sheetName, "N", "N", 12)  // DPT
	for i := 0; i < len(partaiList); i++ {
		colName, _ := excelize.ColumnNumberToName(15 + i)
		f.SetColWidth(sheetName, colName, colName, 12)
	}

	// Write data
	rowNum := 2
	for idx, kel := range kelurahanList {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), idx+1)         // NO
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), proName)       // PROVINSI
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), kel.ProKode)   // KODE PROV
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), kel.DapilNama) // DAPIL
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), kel.DapilKode) // KODE DAPIL
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), kel.KabNama)   // KAB/KOTA
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), kel.KabKode)   // KODE KAB
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowNum), kel.KecNama)   // KECAMATAN
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowNum), kel.KecKode)   // KODE KEC
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", rowNum), kel.KelNama)   // KELURAHAN/DESA
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", rowNum), kel.KelKode)   // KODE DESA

		if tps, ok := tpsData[kel.KelKode]; ok {
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), tps.JmlTPS) // TPS
			f.SetCellValue(sheetName, fmt.Sprintf("M%d", rowNum), "")         // KODE TPS (empty for perdesa)
			f.SetCellValue(sheetName, fmt.Sprintf("N%d", rowNum), tps.JmlDPT) // DPT
		} else {
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), 0)
			f.SetCellValue(sheetName, fmt.Sprintf("M%d", rowNum), "")
			f.SetCellValue(sheetName, fmt.Sprintf("N%d", rowNum), 0)
		}

		// Write party votes (starting from column O = 15)
		for i, partai := range partaiList {
			col := 15 + i
			cell, _ := excelize.CoordinatesToCellName(col, rowNum)
			if partaiVotes, ok := partaiData[kel.KelKode]; ok {
				if data, exists := partaiVotes[partai.NomorUrut]; exists {
					f.SetCellValue(sheetName, cell, data.JmlSuaraTotal)
				} else {
					f.SetCellValue(sheetName, cell, 0)
				}
			} else {
				f.SetCellValue(sheetName, cell, 0)
			}
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if len(kelurahanList) > 0 {
		lastRow := len(kelurahanList) + 1
		lastCol, _ := excelize.ColumnNumberToName(len(headers))
		f.SetCellStyle(sheetName, "A2", lastCol+strconv.Itoa(lastRow), dataStyle)
	}

	filename := fmt.Sprintf("DPRD_Prov_Partai_Per_Kelurahan_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadDapilPilpres handles Excel download for Pilpres data by dapil
func (h *DPRDownloadHandler) DownloadDapilPilpres(c echo.Context) error {
	dapilCode := c.Param("code")

	// Get dapil info
	var dapilID, dapilName, proCode, proName string
	err := h.db.QueryRow("SELECT dapil_id, dapil_nama, pro_kode FROM pdpr_wil_dapil WHERE dapil_kode = ?", dapilCode).Scan(&dapilID, &dapilName, &proCode)
	if err != nil {
		return c.String(http.StatusNotFound, "Dapil tidak ditemukan")
	}

	// Get province name
	err = h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = "Unknown"
	}

	// Get all kelurahan in this dapil
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama
		FROM pdpr_wil_kel k
		JOIN pdpr_wil_kec kec ON k.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON kec.kab_kode = kab.kab_kode
		WHERE k.dapil_kode = ?
		ORDER BY kab.kab_nama, kec.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelurahanQuery, dapilCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data")
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
	}

	var kelurahanList []KelurahanInfo
	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
	}

	// Get TPS count and DPT sum per kelurahan
	tpsQuery := `SELECT kel_kode, COUNT(*) as jml_tps, SUM(COALESCE(total_dpt, 0)) as jml_dpt FROM pdpr_wil_tps WHERE dapil_kode = ? GROUP BY kel_kode`
	tpsRows, err := h.db.Query(tpsQuery, dapilCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS, jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err != nil {
			continue
		}
		tpsData[kelKode] = struct {
			JmlTPS int
			JmlDPT int
		}{jmlTPS, jmlDPT}
	}

	// Get pilpres vote data from hs_pilpres_kel
	// First, get paslon mapping from ppwp table
	pasalonMap := make(map[string]int) // paslon_id -> nomor_urut
	pasalonRows, err := h.db.Query("SELECT id, nomor_urut FROM ppwp ORDER BY nomor_urut")
	if err == nil {
		defer pasalonRows.Close()
		for pasalonRows.Next() {
			var id string
			var nomorUrut int
			if err := pasalonRows.Scan(&id, &nomorUrut); err == nil {
				pasalonMap[id] = nomorUrut
			}
		}
	}

	voteDataQuery := `SELECT kel_kode, chart FROM hs_pilpres_kel WHERE pro_kode = ?`
	voteRows, err := h.db.Query(voteDataQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	type PasalonData struct {
		Nomor      int
		JumlahSuara int
	}

	pasalonVotes := make(map[string]map[int]int) // kel_kode -> nomor_urut -> jumlah_suara
	for voteRows.Next() {
		var kelKode, chartJSON string
		if err := voteRows.Scan(&kelKode, &chartJSON); err != nil {
			continue
		}

		var chartData map[string]interface{}
		if err := json.Unmarshal([]byte(chartJSON), &chartData); err != nil {
			continue
		}

		pasalonVotes[kelKode] = make(map[int]int)
		for pasalonID, votes := range chartData {
			if pasalonID == "persen" {
				continue
			}
			if nomorUrut, ok := pasalonMap[pasalonID]; ok {
				if votesFloat, ok := votes.(float64); ok {
					pasalonVotes[kelKode][nomorUrut] = int(votesFloat)
				}
			}
		}
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "PILPRES"
	f.SetSheetName("Sheet1", sheetName)

	// Headers
	headers := []string{
		"NO", "KODE PROV", "KODE DAPIL", "KODE KAB", "KODE KEC", "KODE DESA",
		"DAPIL", "KABUPATEN/KOTA", "KECAMATAN", "KELURAHAN/DESA",
		"JML TPS", "JML DPT", "PASLON 1", "PASLON 2", "PASLON 3",
	}

	// Write headers with styling
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "F", 12)
	f.SetColWidth(sheetName, "G", "J", 25)
	f.SetColWidth(sheetName, "K", "O", 12)

	// Write data
	rowNum := 2
	for idx, kel := range kelurahanList {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), idx+1)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), kel.ProKode)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), kel.DapilKode)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), kel.KabKode)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), kel.KecKode)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), kel.KelKode)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), dapilName)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowNum), kel.KabNama)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowNum), kel.KecNama)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", rowNum), kel.KelNama)

		if tps, ok := tpsData[kel.KelKode]; ok {
			f.SetCellValue(sheetName, fmt.Sprintf("K%d", rowNum), tps.JmlTPS)
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), tps.JmlDPT)
		} else {
			f.SetCellValue(sheetName, fmt.Sprintf("K%d", rowNum), 0)
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), 0)
		}


		// Write paslon votes (paslon 1, 2, 3)
		if paslonData, ok := pasalonVotes[kel.KelKode]; ok {
			for nomorUrut := 1; nomorUrut <= 3; nomorUrut++ {
				col := 12 + nomorUrut
				cell, _ := excelize.CoordinatesToCellName(col, rowNum)
				if suara, exists := paslonData[nomorUrut]; exists {
					f.SetCellValue(sheetName, cell, suara)
				} else {
					f.SetCellValue(sheetName, cell, 0)
				}
			}
		} else {
			for nomorUrut := 1; nomorUrut <= 3; nomorUrut++ {
				col := 12 + nomorUrut
				cell, _ := excelize.CoordinatesToCellName(col, rowNum)
				f.SetCellValue(sheetName, cell, 0)
			}
		}









		rowNum++
	}

	filename := fmt.Sprintf("PILPRES_Per_Kelurahan_Dapil_%s.xlsx", dapilName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadDapilDPRRIPartai handles Excel download for DPR RI party data by dapil (per kelurahan)
func (h *DPRDownloadHandler) DownloadDapilDPRRIPartai(c echo.Context) error {
	dapilCode := c.Param("code")
	// Reuse existing handler by passing dapil code as "id"
	c.SetParamNames("id")
	c.SetParamValues(dapilCode)
	return h.DownloadDPRRIPartai(c)
}

// DownloadDapilDPRRICaleg handles Excel download for DPR RI caleg data by dapil (per kelurahan)
func (h *DPRDownloadHandler) DownloadDapilDPRRICaleg(c echo.Context) error {
	dapilCode := c.Param("code")
	// Reuse existing handler by passing dapil code as "id"
	c.SetParamNames("id")
	c.SetParamValues(dapilCode)
	return h.DownloadDPRRICaleg(c)
}

// DownloadDapilDPRRIPartaiTPS handles Excel download for DPR RI party data by dapil (per TPS)
func (h *DPRDownloadHandler) DownloadDapilDPRRIPartaiTPS(c echo.Context) error {
	dapilCode := c.Param("code")

	// Get dapil info
	var dapilID, dapilName, proCode string
	err := h.db.QueryRow("SELECT dapil_id, dapil_nama, pro_kode FROM pdpr_wil_dapil WHERE dapil_kode = ?", dapilCode).Scan(&dapilID, &dapilName, &proCode)
	if err != nil {
		return c.String(http.StatusNotFound, "Dapil tidak ditemukan")
	}

	// Get all TPS in this dapil from pdpr_wil_tps
	tpsQuery := `
		SELECT DISTINCT
			t.tps_id, t.tps_kode, t.tps_nama,
			t.pro_kode, t.dapil_kode, t.kab_kode, t.kec_kode, t.kel_kode,
			kel.kel_nama, kec.kec_nama, kab.kab_nama,
			COALESCE(t.total_dpt, 0) as total_dpt
		FROM pdpr_wil_tps t
		JOIN pdpr_wil_kel kel ON t.kel_kode = kel.kel_kode
		JOIN pdpr_wil_kec kec ON t.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON t.kab_kode = kab.kab_kode
		WHERE t.dapil_kode = ?
		ORDER BY kab.kab_nama, kec.kec_nama, kel.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, dapilCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data: "+err.Error())
	}
	defer tpsRows.Close()

	type TPSInfo struct {
		TPSID     string
		TPSKode   string
		TPSNama   string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
		TotalDPT  int
	}

	var tpsList []TPSInfo
	for tpsRows.Next() {
		var t TPSInfo
		if err := tpsRows.Scan(&t.TPSID, &t.TPSKode, &t.TPSNama,
			&t.ProKode, &t.DapilKode, &t.KabKode, &t.KecKode, &t.KelKode,
			&t.KelNama, &t.KecNama, &t.KabNama, &t.TotalDPT); err != nil {
			continue
		}
		tpsList = append(tpsList, t)
	}

	if len(tpsList) == 0 {
		return c.String(http.StatusNotFound, "Tidak ada data TPS untuk dapil ini")
	}

	// Get party list from dapil's candidates
	partaiQuery := `
		SELECT DISTINCT p.id, p.nama, p.partai_singkat, p.nomor_urut
		FROM partai p
		INNER JOIN dpr_ri_caleg c ON p.id = c.partai_id
		WHERE c.dapil_id = ?
		ORDER BY p.nomor_urut
	`
	partaiRows, err := h.db.Query(partaiQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying party data: "+err.Error())
	}
	defer partaiRows.Close()

	type PartaiInfo struct {
		ID            int
		Nama          string
		PartaiSingkat string
		NomorUrut     int
	}

	var partaiList []PartaiInfo
	for partaiRows.Next() {
		var p PartaiInfo
		if err := partaiRows.Scan(&p.ID, &p.Nama, &p.PartaiSingkat, &p.NomorUrut); err != nil {
			continue
		}
		partaiList = append(partaiList, p)
	}

	// Get vote data from hr_dpr_ri_kel for this dapil
	voteDataQuery := `SELECT kel_kode, tbl FROM hr_dpr_ri_kel WHERE dapil_id = ?`
	voteRows, err := h.db.Query(voteDataQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data: "+err.Error())
	}
	defer voteRows.Close()

	// Parse TPS-level data from tbl field
	// tbl contains: {"tps_kode": {"caleg_id": votes, ...}, ...}
	tpsVoteData := make(map[string]map[string]int) // tps_kode -> caleg_id -> votes

	for voteRows.Next() {
		var kelKode, tblJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON); err != nil {
			continue
		}

		// Parse TPS data: {"tps_kode": {"caleg_id": votes, ...}, ...}
		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		for tpsKode, calegVotes := range tpsDataMap {
			if _, exists := tpsVoteData[tpsKode]; !exists {
				tpsVoteData[tpsKode] = make(map[string]int)
			}

			for calegID, voteVal := range calegVotes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				tpsVoteData[tpsKode][calegID] = votes
			}
		}
	}

	// Get caleg ID to party mapping
	calegPartyQuery := `
		SELECT c.id, c.partai_id
		FROM dpr_ri_caleg c
		WHERE c.dapil_id = ?
	`
	calegRows, err := h.db.Query(calegPartyQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying caleg data: "+err.Error())
	}
	defer calegRows.Close()

	calegPartyMap := make(map[string]int) // caleg_id -> party_id
	for calegRows.Next() {
		var calegID string
		var partaiID int
		if err := calegRows.Scan(&calegID, &partaiID); err != nil {
			continue
		}
		calegPartyMap[calegID] = partaiID
	}

	// Aggregate votes by TPS and party
	tpsPartaiVotes := make(map[string]map[int]int) // tps_kode -> partai_id -> total_votes
	for tpsKode, calegVotes := range tpsVoteData {
		if _, exists := tpsPartaiVotes[tpsKode]; !exists {
			tpsPartaiVotes[tpsKode] = make(map[int]int)
		}

		for calegID, votes := range calegVotes {
			if partaiID, ok := calegPartyMap[calegID]; ok {
				tpsPartaiVotes[tpsKode][partaiID] += votes
			}
		}
	}

	// Get province name
	var proName string
	err = h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = ""
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Partai Per TPS"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Set headers
	headers := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	for _, p := range partaiList {
		headers = append(headers, p.PartaiSingkat)
	}
	headers = append(headers, "TOTAL")

	// Write headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Style headers
	lastCol, _ := excelize.ColumnNumberToName(len(headers))
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Write data rows
	rowNum := 2
	for idx, tps := range tpsList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.ProKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilName)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelKode)
		colNum++

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSNama)
		colNum++

		// KODE TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSKode)
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TotalDPT)
		colNum++

		// Party votes
		totalSuara := 0
		tpsVotes := tpsPartaiVotes[tps.TPSKode]
		for _, partai := range partaiList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			votes := tpsVotes[partai.ID]
			if votes > 0 || len(tpsVotes) > 0 {
				f.SetCellValue(sheetName, cell, votes)
				totalSuara += votes
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if len(tpsVotes) > 0 {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	lastRow := rowNum - 1
	lastColName, _ := excelize.ColumnNumberToName(len(headers))
	f.SetCellStyle(sheetName, "A2", lastColName+fmt.Sprintf("%d", lastRow), dataStyle)

	filename := fmt.Sprintf("DPR_RI_Partai_Per_TPS_Dapil_%s.xlsx", dapilName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadDapilDPRRICalegTPS handles Excel download for DPR RI caleg data by dapil (per TPS)
func (h *DPRDownloadHandler) DownloadDapilDPRRICalegTPS(c echo.Context) error {
	dapilCode := c.Param("code")

	// Get dapil info
	var dapilID, dapilName, proCode string
	err := h.db.QueryRow("SELECT dapil_id, dapil_nama, pro_kode FROM pdpr_wil_dapil WHERE dapil_kode = ?", dapilCode).Scan(&dapilID, &dapilName, &proCode)
	if err != nil {
		return c.String(http.StatusNotFound, "Dapil tidak ditemukan")
	}

	// Get all TPS in this dapil
	tpsQuery := `
		SELECT DISTINCT
			t.tps_id, t.tps_kode, t.tps_nama,
			t.pro_kode, t.dapil_kode, t.kab_kode, t.kec_kode, t.kel_kode,
			kel.kel_nama, kec.kec_nama, kab.kab_nama,
			COALESCE(t.total_dpt, 0) as total_dpt
		FROM pdpr_wil_tps t
		JOIN pdpr_wil_kel kel ON t.kel_kode = kel.kel_kode
		JOIN pdpr_wil_kec kec ON t.kec_kode = kec.kec_kode
		JOIN pdpr_wil_kab kab ON t.kab_kode = kab.kab_kode
		WHERE t.dapil_kode = ?
		ORDER BY kab.kab_nama, kec.kec_nama, kel.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, dapilCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	type TPSInfo struct {
		TPSID     string
		TPSKode   string
		TPSNama   string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
		TotalDPT  int
	}

	var tpsList []TPSInfo
	for tpsRows.Next() {
		var t TPSInfo
		if err := tpsRows.Scan(&t.TPSID, &t.TPSKode, &t.TPSNama,
			&t.ProKode, &t.DapilKode, &t.KabKode, &t.KecKode, &t.KelKode,
			&t.KelNama, &t.KecNama, &t.KabNama, &t.TotalDPT); err != nil {
			continue
		}
		tpsList = append(tpsList, t)
	}

	if len(tpsList) == 0 {
		return c.String(http.StatusNotFound, "Tidak ada data TPS untuk dapil ini")
	}

	// Get vote data from hr_dpr_ri_kel for this dapil
	voteDataQuery := `SELECT kel_kode, tbl FROM hr_dpr_ri_kel WHERE dapil_id = ?`
	voteRows, err := h.db.Query(voteDataQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data: "+err.Error())
	}
	defer voteRows.Close()

	// Parse TPS-level data from tbl field
	// tbl contains: {"tps_kode": {"caleg_id": votes, ...}, ...}
	tpsVoteData := make(map[string]map[string]int) // tps_kode -> caleg_id -> votes

	for voteRows.Next() {
		var kelKode, tblJSON string
		if err := voteRows.Scan(&kelKode, &tblJSON); err != nil {
			continue
		}

		// Parse TPS data: {"tps_kode": {"caleg_id": votes, ...}, ...}
		var tpsDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tpsDataMap); err != nil {
			continue
		}

		for tpsKode, calegVotes := range tpsDataMap {
			if _, exists := tpsVoteData[tpsKode]; !exists {
				tpsVoteData[tpsKode] = make(map[string]int)
			}

			for calegID, voteVal := range calegVotes {
				if calegID == "null" {
					continue
				}
				votes := 0
				switch v := voteVal.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				case string:
					votes, _ = strconv.Atoi(v)
				}
				tpsVoteData[tpsKode][calegID] = votes
			}
		}
	}

	// Get all candidates for this dapil
	candidateQuery := `
		SELECT c.id, c.nama, p.nama as nama_partai, p.partai_singkat, c.nomor_urut, c.partai_id
		FROM dpr_ri_caleg c
		LEFT JOIN partai p ON c.partai_id = p.id
		WHERE c.dapil_id = ?
		ORDER BY p.nomor_urut, c.nomor_urut
	`
	calegRows, err := h.db.Query(candidateQuery, dapilID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying candidate data: "+err.Error())
	}
	defer calegRows.Close()

	type Candidate struct {
		ID            string
		Nama          string
		Partai        string
		PartaiSingkat string
		NomorUrut     int
		PartaiID      int
	}

	var candidatesList []Candidate
	for calegRows.Next() {
		var c Candidate
		if err := calegRows.Scan(&c.ID, &c.Nama, &c.Partai, &c.PartaiSingkat, &c.NomorUrut, &c.PartaiID); err != nil {
			continue
		}
		candidatesList = append(candidatesList, c)
	}

	// Get province name
	var proName string
	err = h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = ""
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Suara Caleg Per TPS"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating Excel sheet")
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Fixed columns
	fixedCols := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Calculate total columns: fixed + candidates + total
	totalCols := len(fixedCols) + len(candidatesList) + 1

	// Write headers
	colNum := 1
	for _, col := range fixedCols {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, col)
		colNum++
	}

	// Add candidate columns
	for _, cand := range candidatesList {
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		headerText := fmt.Sprintf("%s\n%d\n%s", cand.PartaiSingkat, cand.NomorUrut, cand.Nama)
		f.SetCellValue(sheetName, cell, headerText)
		colNum++
	}

	// TOTAL column
	cell, _ := excelize.CoordinatesToCellName(colNum, 1)
	f.SetCellValue(sheetName, cell, "TOTAL")

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	lastCol, _ := excelize.ColumnNumberToName(totalCols)
	f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 20)
	f.SetColWidth(sheetName, "C", "C", 12)
	f.SetColWidth(sheetName, "D", "D", 25)
	f.SetRowHeight(sheetName, 1, 40)

	// Write data rows
	rowNum := 2
	for idx, tps := range tpsList {
		colNum := 1

		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, idx+1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proName)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.ProKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilName)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.DapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.KelKode)
		colNum++

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSNama)
		colNum++

		// KODE TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TPSKode)
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tps.TotalDPT)
		colNum++

		// Candidate votes
		totalSuara := 0
		tpsVotes := tpsVoteData[tps.TPSKode]
		for _, cand := range candidatesList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			votes := 0
			if tpsVotes != nil {
				if v, ok := tpsVotes[cand.ID]; ok {
					votes = v
				}
			}
			if len(tpsVotes) > 0 {
				f.SetCellValue(sheetName, cell, votes)
				totalSuara += votes
			} else {
				f.SetCellValue(sheetName, cell, "-")
			}
			colNum++
		}

		// TOTAL column
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if len(tpsVotes) > 0 {
			f.SetCellValue(sheetName, cell, totalSuara)
		} else {
			f.SetCellValue(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	lastRow := rowNum - 1
	if lastRow >= 2 {
		f.SetCellStyle(sheetName, "A2", lastCol+fmt.Sprintf("%d", lastRow), dataStyle)
	}

	filename := fmt.Sprintf("DPR_RI_Caleg_Per_TPS_Dapil_%s.xlsx", dapilName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadProvinsiDPRDProvCaleg handles Excel download for DPRD Provinsi caleg data per kelurahan
func (h *DPRDownloadHandler) DownloadProvinsiDPRDProvCaleg(c echo.Context) error {
	proCode := c.Param("code")

	// Get province name
	var proName string
	err := h.db.QueryRow("SELECT pro_nama FROM pdprdp_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all dapil in this province
	type DapilInfo struct {
		DapilID   int
		DapilKode string
		DapilNama string
	}
	var dapilList []DapilInfo
	dapilRows, err := h.db.Query(`
		SELECT DISTINCT dapil_id, dapil_kode, dapil_nama
		FROM dprd_pro_dapil
		WHERE pro_kode = ?
		ORDER BY dapil_nama
	`, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying dapil data")
	}
	defer dapilRows.Close()

	for dapilRows.Next() {
		var d DapilInfo
		if err := dapilRows.Scan(&d.DapilID, &d.DapilKode, &d.DapilNama); err == nil {
			dapilList = append(dapilList, d)
		}
	}

	if len(dapilList) == 0 {
		return c.String(http.StatusNotFound, "Tidak ada dapil untuk provinsi ini")
	}

	// Get all kelurahan in this province
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdprdp_wil_kel k
		JOIN pdprdp_wil_kec kec ON k.kec_kode = kec.kec_kode AND k.kab_kode = kec.kab_kode
		JOIN pdprdp_wil_kab kab ON k.kab_kode = kab.kab_kode
		JOIN dprd_pro_dapil d ON k.dapil_id = d.dapil_id
		WHERE k.pro_kode = ?
		ORDER BY d.dapil_nama, kab.kab_nama, kec.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelurahanQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data: "+err.Error())
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	kelurahanByDapil := make(map[string][]KelurahanInfo) // dapil_id -> kelurahan list
	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama, &k.DapilNama); err != nil {
			continue
		}
		kelurahanByDapil[k.DapilID] = append(kelurahanByDapil[k.DapilID], k)
	}

	// Get TPS count and DPT sum per kelurahan
	tpsQuery := `
		SELECT kel_kode, COUNT(*) as jml_tps, COALESCE(SUM(total_dpt), 0) as jml_dpt
		FROM pdpr_wil_tps
		WHERE pro_kode = ?
		GROUP BY kel_kode
	`

	tpsRows, err := h.db.Query(tpsQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS int
		var jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err == nil {
			tpsData[kelKode] = struct {
				JmlTPS int
				JmlDPT int
			}{JmlTPS: jmlTPS, JmlDPT: jmlDPT}
		}
	}

	// Get party information
	partaiMap := make(map[string]string) // partai_id -> partai_singkat
	partaiRows, err := h.db.Query(`SELECT id, partai_singkat FROM partai`)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying partai data")
	}
	defer partaiRows.Close()
	for partaiRows.Next() {
		var id string
		var singkat string
		if err := partaiRows.Scan(&id, &singkat); err == nil {
			partaiMap[id] = singkat
		}
	}

	// Get caleg list for this province sorted by dapil and nomor urut
	calegRows, err := h.db.Query(`
		SELECT c.id, c.nama, c.nomor_urut, c.partai_id, c.dapil_id, c.dapil_nama
		FROM dprd_pro_caleg c
		LEFT JOIN partai p ON c.partai_id = p.id
		WHERE c.pro_kode = ?
		ORDER BY c.dapil_id, p.nomor_urut, c.nomor_urut
	`, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying caleg data")
	}
	defer calegRows.Close()

	type CalegInfo struct {
		ID         int
		Nama       string
		NomorUrut  int
		PartaiID   string
		PartaiName string
		DapilID    int
		DapilNama  string
	}

	calegByDapil := make(map[int][]CalegInfo) // dapil_id -> caleg list
	for calegRows.Next() {
		var c CalegInfo
		if err := calegRows.Scan(&c.ID, &c.Nama, &c.NomorUrut, &c.PartaiID, &c.DapilID, &c.DapilNama); err != nil {
			continue
		}
		c.PartaiName = partaiMap[c.PartaiID]
		if c.PartaiName == "" {
			c.PartaiName = "-"
		}
		calegByDapil[c.DapilID] = append(calegByDapil[c.DapilID], c)
	}

	// Get vote data from hr_dprd_pro_kec.tbl
	// tbl format: {"kel_kode": {"caleg_id": vote_count, ...}, ...}
	voteDataQuery := `SELECT kec_kode, tbl FROM hr_dprd_pro_kec WHERE pro_kode = ?`
	voteRows, err := h.db.Query(voteDataQuery, proCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data")
	}
	defer voteRows.Close()

	// Parse vote data: kec_kode -> {kel_kode -> {caleg_id -> votes}}
	calegVoteData := make(map[string]map[int]int) // kel_kode -> caleg_id -> votes

	for voteRows.Next() {
		var kecKode, tblJSON string
		if err := voteRows.Scan(&kecKode, &tblJSON); err != nil {
			continue
		}

		// Parse TBL JSON: {"kel_kode": {"caleg_id": votes, ...}, ...}
		var tblData map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(tblJSON), &tblData); err != nil {
			continue
		}

		for kelKode, calegVotes := range tblData {
			if calegVoteData[kelKode] == nil {
				calegVoteData[kelKode] = make(map[int]int)
			}

			for calegIDStr, votesInterface := range calegVotes {
				if calegIDStr == "null" {
					continue
				}

				calegID, err := strconv.Atoi(calegIDStr)
				if err != nil {
					continue
				}

				var votes int
				switch v := votesInterface.(type) {
				case float64:
					votes = int(v)
				case int:
					votes = v
				default:
					continue
				}

				calegVoteData[kelKode][calegID] = votes
			}
		}
	}

	// Create Excel file
	f := excelize.NewFile()

	// Header styles
	headerRow1Style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	headerRow2Style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"5B9BD5"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	headerRow3Style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"70AD47"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Create one sheet per dapil
	for _, dapil := range dapilList {
		sheetName := dapil.DapilNama
		// Sanitize sheet name (Excel limitation: max 31 chars, no special chars)
		if len(sheetName) > 31 {
			sheetName = sheetName[:31]
		}

		idx, err := f.NewSheet(sheetName)
		if err != nil {
			continue
		}
		if idx == 0 {
			f.SetActiveSheet(idx)
		}

		// Get kelurahan for this dapil
		kelurahanList := kelurahanByDapil[strconv.Itoa(dapil.DapilID)]
		if len(kelurahanList) == 0 {
			continue
		}

		// Get caleg for this dapil
		calegList := calegByDapil[dapil.DapilID]
		if len(calegList) == 0 {
			continue
		}

		// Fixed columns
		fixedHeaders := []string{
			"NO", "KODE PROV", "KODE DAPIL", "KODE KAB", "KODE KEC", "KODE DESA",
			"DAPIL", "KABUPATEN/KOTA", "KECAMATAN", "KELURAHAN/DESA",
			"JML TPS", "JML DPT",
		}

		// Row 1: PARTAI
		for i, header := range fixedHeaders {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, header)
			f.SetCellStyle(sheetName, cell, cell, headerRow1Style)
			// Merge rows 1-3 for fixed columns
			endCell, _ := excelize.CoordinatesToCellName(i+1, 3)
			f.MergeCell(sheetName, cell, endCell)
		}
		for i, caleg := range calegList {
			col := len(fixedHeaders) + 1 + i
			cell, _ := excelize.CoordinatesToCellName(col, 1)
			f.SetCellValue(sheetName, cell, caleg.PartaiName)
			f.SetCellStyle(sheetName, cell, cell, headerRow1Style)
		}

		// Add "JUMLAH SUARA" column header
		totalCol := len(fixedHeaders) + len(calegList) + 1
		totalCell, _ := excelize.CoordinatesToCellName(totalCol, 1)
		f.SetCellValue(sheetName, totalCell, "JUMLAH SUARA")
		f.SetCellStyle(sheetName, totalCell, totalCell, headerRow1Style)
		totalEndCell, _ := excelize.CoordinatesToCellName(totalCol, 3)
		f.MergeCell(sheetName, totalCell, totalEndCell)

		// Row 2: NOMOR URUT CALEG
		for i, caleg := range calegList {
			col := len(fixedHeaders) + 1 + i
			cell, _ := excelize.CoordinatesToCellName(col, 2)
			f.SetCellValue(sheetName, cell, caleg.NomorUrut)
			f.SetCellStyle(sheetName, cell, cell, headerRow2Style)
		}

		// Row 3: NAMA CALEG
		for i, caleg := range calegList {
			col := len(fixedHeaders) + 1 + i
			cell, _ := excelize.CoordinatesToCellName(col, 3)
			f.SetCellValue(sheetName, cell, caleg.Nama)
			f.SetCellStyle(sheetName, cell, cell, headerRow3Style)
		}

		// Set column widths
		f.SetColWidth(sheetName, "A", "A", 5)
		f.SetColWidth(sheetName, "B", "F", 12)
		f.SetColWidth(sheetName, "G", "J", 25)
		f.SetColWidth(sheetName, "K", "L", 10)
		for i := 0; i < len(calegList); i++ {
			colName, _ := excelize.ColumnNumberToName(len(fixedHeaders) + 1 + i)
			f.SetColWidth(sheetName, colName, colName, 20)
		}
		// Set width for JUMLAH SUARA column
		totalColName, _ := excelize.ColumnNumberToName(totalCol)
		f.SetColWidth(sheetName, totalColName, totalColName, 15)

		// Set row height for header rows
		f.SetRowHeight(sheetName, 1, 30)
		f.SetRowHeight(sheetName, 2, 25)
		f.SetRowHeight(sheetName, 3, 25)

		// Write data starting from row 4
		rowNum := 4
		for idx, kel := range kelurahanList {
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), idx+1)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), kel.ProKode)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), kel.DapilKode)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), kel.KabKode)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), kel.KecKode)
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), kel.KelKode)
			f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), kel.DapilNama)
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowNum), kel.KabNama)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowNum), kel.KecNama)
			f.SetCellValue(sheetName, fmt.Sprintf("J%d", rowNum), kel.KelNama)

			if tps, ok := tpsData[kel.KelKode]; ok {
				f.SetCellValue(sheetName, fmt.Sprintf("K%d", rowNum), tps.JmlTPS)
				f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), tps.JmlDPT)
			} else {
				f.SetCellValue(sheetName, fmt.Sprintf("K%d", rowNum), "-")
				f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), "-")
			}

			// Write caleg votes and calculate total
			totalVotes := 0
			for i, caleg := range calegList {
				col := len(fixedHeaders) + 1 + i
				cell, _ := excelize.CoordinatesToCellName(col, rowNum)
				if votes, ok := calegVoteData[kel.KelKode]; ok {
					if suara, exists := votes[caleg.ID]; exists {
						f.SetCellValue(sheetName, cell, suara)
						totalVotes += suara
					} else {
						f.SetCellValue(sheetName, cell, 0)
					}
				} else {
					f.SetCellValue(sheetName, cell, 0)
				}
			}

			// Write total votes
			totalVotesCell, _ := excelize.CoordinatesToCellName(totalCol, rowNum)
			f.SetCellValue(sheetName, totalVotesCell, totalVotes)

			rowNum++
		}

		// Apply borders to all data cells
		if len(kelurahanList) > 0 {
			lastRow := len(kelurahanList) + 3
			lastCol, _ := excelize.ColumnNumberToName(len(fixedHeaders) + len(calegList) + 1)
			f.SetCellStyle(sheetName, "A4", lastCol+strconv.Itoa(lastRow), dataStyle)
		}
	}

	// Delete default Sheet1 after creating all sheets
	f.DeleteSheet("Sheet1")

	filename := fmt.Sprintf("DPRD_Prov_Caleg_Per_Dapil_%s.xlsx", proName)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadDPRDKabPartai handles Excel download for DPRD Kabupaten party data
func (h *DPRDownloadHandler) DownloadDPRDKabPartai(c echo.Context) error {
	kabCode := c.Param("code")

	// Get kabupaten name and province code
	var kabNama, proCode, proName string
	err := h.db.QueryRow("SELECT kab_nama, pro_kode FROM pdprdk_wil_kab WHERE kab_kode = ?", kabCode).Scan(&kabNama, &proCode)
	if err != nil {
		return c.String(http.StatusNotFound, "Kabupaten tidak ditemukan")
	}

	// Get province name
	err = h.db.QueryRow("SELECT pro_nama FROM pdprdk_wil_pro WHERE pro_kode = ?", proCode).Scan(&proName)
	if err != nil {
		proName = "Unknown"
	}

	// Get all kelurahan in this kabupaten with JOIN to get names
	kelurahanQuery := `
		SELECT DISTINCT
			k.pro_id, k.dapil_id, k.kab_id, k.kec_id, k.kel_id,
			k.pro_kode, k.dapil_kode, k.kab_kode, k.kec_kode, k.kel_kode,
			k.kel_nama, kec.kec_nama, kab.kab_nama, d.dapil_nama
		FROM pdprdk_wil_kel k
		JOIN pdprdk_wil_kec kec ON k.kec_kode = kec.kec_kode AND k.kab_kode = kec.kab_kode
		JOIN pdprdk_wil_kab kab ON k.kab_kode = kab.kab_kode
		JOIN dprd_kab_dapil d ON k.dapil_id = d.dapil_id
		WHERE k.kab_kode = ?
		ORDER BY d.dapil_nama, kec.kec_nama, k.kel_nama
	`

	kelRows, err := h.db.Query(kelurahanQuery, kabCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying kelurahan data: "+err.Error())
	}
	defer kelRows.Close()

	type KelurahanInfo struct {
		ProID     string
		DapilID   string
		KabID     string
		KecID     string
		KelID     string
		ProKode   string
		DapilKode string
		KabKode   string
		KecKode   string
		KelKode   string
		KelNama   string
		KecNama   string
		KabNama   string
		DapilNama string
	}

	var kelurahanList []KelurahanInfo
	for kelRows.Next() {
		var k KelurahanInfo
		if err := kelRows.Scan(&k.ProID, &k.DapilID, &k.KabID, &k.KecID, &k.KelID,
			&k.ProKode, &k.DapilKode, &k.KabKode, &k.KecKode, &k.KelKode,
			&k.KelNama, &k.KecNama, &k.KabNama, &k.DapilNama); err != nil {
			continue
		}
		kelurahanList = append(kelurahanList, k)
	}

	if len(kelurahanList) == 0 {
		return c.String(http.StatusNotFound, "Tidak ada data kelurahan untuk kabupaten ini")
	}

	// Get TPS count and DPT sum per kelurahan
	tpsQuery := `
		SELECT kel_kode, COUNT(*) as jml_tps, COALESCE(SUM(total_dpt), 0) as jml_dpt
		FROM pdpr_wil_tps
		WHERE kab_kode = ?
		GROUP BY kel_kode
	`

	tpsRows, err := h.db.Query(tpsQuery, kabCode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data: "+err.Error())
	}
	defer tpsRows.Close()

	tpsData := make(map[string]struct {
		JmlTPS int
		JmlDPT int
	})
	for tpsRows.Next() {
		var kelKode string
		var jmlTPS int
		var jmlDPT int
		if err := tpsRows.Scan(&kelKode, &jmlTPS, &jmlDPT); err == nil {
			tpsData[kelKode] = struct {
				JmlTPS int
				JmlDPT int
			}{JmlTPS: jmlTPS, JmlDPT: jmlDPT}
		}
	}

	// Get party vote data from hr_dprd_kab_kel using IN clause
	kelKodes := make([]interface{}, len(kelurahanList))
	for i, k := range kelurahanList {
		kelKodes[i] = k.KelKode
	}

	// Build placeholders for IN clause
	placeholders := ""
	for i := range kelKodes {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}

	voteQuery := fmt.Sprintf(`
		SELECT kel_kode, chart
		FROM hr_dprd_kab_kel
		WHERE kab_kode = ? AND kel_kode IN (%s)
	`, placeholders)

	args := make([]interface{}, len(kelKodes)+1)
	args[0] = kabCode
	copy(args[1:], kelKodes)

	voteRows, err := h.db.Query(voteQuery, args...)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying vote data: "+err.Error())
	}
	defer voteRows.Close()

	// Parse party data from chart JSON
	partaiData := make(map[string]map[int]struct {
		JmlSuaraTotal  int
		JmlSuaraPartai int
	})

	for voteRows.Next() {
		var kelKode string
		var chartJSON sql.NullString
		if err := voteRows.Scan(&kelKode, &chartJSON); err != nil {
			continue
		}

		if !chartJSON.Valid || chartJSON.String == "" {
			continue
		}

		var partaiDataMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(chartJSON.String), &partaiDataMap); err != nil {
			continue
		}

		if _, exists := partaiData[kelKode]; !exists {
			partaiData[kelKode] = make(map[int]struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			})
		}

		for nomorUrutStr, data := range partaiDataMap {
			nomorUrut, _ := strconv.Atoi(nomorUrutStr)
			jmlSuaraTotal := 0
			jmlSuaraPartai := 0

			if val, ok := data["jml_suara_total"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraTotal = int(v)
				case int:
					jmlSuaraTotal = v
				case string:
					jmlSuaraTotal, _ = strconv.Atoi(v)
				}
			}

			if val, ok := data["jml_suara_partai"]; ok {
				switch v := val.(type) {
				case float64:
					jmlSuaraPartai = int(v)
				case int:
					jmlSuaraPartai = v
				case string:
					jmlSuaraPartai, _ = strconv.Atoi(v)
				}
			}

			partaiData[kelKode][nomorUrut] = struct {
				JmlSuaraTotal  int
				JmlSuaraPartai int
			}{JmlSuaraTotal: jmlSuaraTotal, JmlSuaraPartai: jmlSuaraPartai}
		}
	}

	// Get parties list dynamically from database
	// Only include Aceh local parties if province is Aceh (code = '11')
	var partaiQuery string
	if proCode == "11" {
		partaiQuery = "SELECT nomor_urut, partai_singkat FROM partai ORDER BY nomor_urut"
	} else {
		partaiQuery = "SELECT nomor_urut, partai_singkat FROM partai WHERE is_aceh = 'false' OR is_aceh IS NULL ORDER BY nomor_urut"
	}

	partaiRows, err := h.db.Query(partaiQuery)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying partai data: "+err.Error())
	}
	defer partaiRows.Close()

	type PartaiInfo struct {
		NomorUrut     int
		PartaiSingkat string
	}

	var partaiList []PartaiInfo
	for partaiRows.Next() {
		var p PartaiInfo
		if err := partaiRows.Scan(&p.NomorUrut, &p.PartaiSingkat); err == nil {
			partaiList = append(partaiList, p)
		}
	}

	if len(partaiList) == 0 {
		return c.String(http.StatusInternalServerError, "Tidak ada data partai")
	}

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data DPRD Kab Partai"
	f.SetSheetName("Sheet1", sheetName)

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Write headers
	headers := []string{"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL", "KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC", "KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT"}
	for _, partai := range partaiList {
		headers = append(headers, partai.PartaiSingkat)
	}

	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 5)   // NO
	f.SetColWidth(sheetName, "B", "B", 25)  // PROVINSI
	f.SetColWidth(sheetName, "C", "C", 12)  // KODE PROV
	f.SetColWidth(sheetName, "D", "D", 20)  // DAPIL
	f.SetColWidth(sheetName, "E", "E", 12)  // KODE DAPIL
	f.SetColWidth(sheetName, "F", "F", 25)  // KAB/KOTA
	f.SetColWidth(sheetName, "G", "G", 12)  // KODE KAB
	f.SetColWidth(sheetName, "H", "H", 25)  // KECAMATAN
	f.SetColWidth(sheetName, "I", "I", 12)  // KODE KEC
	f.SetColWidth(sheetName, "J", "J", 30)  // KELURAHAN/DESA
	f.SetColWidth(sheetName, "K", "K", 12)  // KODE DESA
	f.SetColWidth(sheetName, "L", "L", 10)  // TPS
	f.SetColWidth(sheetName, "M", "M", 12)  // KODE TPS
	f.SetColWidth(sheetName, "N", "N", 12)  // DPT
	for i := 0; i < len(partaiList); i++ {
		colName, _ := excelize.ColumnNumberToName(15 + i)
		f.SetColWidth(sheetName, colName, colName, 12)
	}

	// Write data
	rowNum := 2
	for idx, kel := range kelurahanList {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), idx+1)         // NO
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), proName)       // PROVINSI
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), kel.ProKode)   // KODE PROV
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), kel.DapilNama) // DAPIL
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), kel.DapilKode) // KODE DAPIL
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), kel.KabNama)   // KAB/KOTA
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), kel.KabKode)   // KODE KAB
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowNum), kel.KecNama)   // KECAMATAN
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowNum), kel.KecKode)   // KODE KEC
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", rowNum), kel.KelNama)   // KELURAHAN/DESA
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", rowNum), kel.KelKode)   // KODE DESA

		if tps, ok := tpsData[kel.KelKode]; ok {
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), tps.JmlTPS) // TPS
			f.SetCellValue(sheetName, fmt.Sprintf("M%d", rowNum), "")         // KODE TPS (empty for perdesa)
			f.SetCellValue(sheetName, fmt.Sprintf("N%d", rowNum), tps.JmlDPT) // DPT
		} else {
			f.SetCellValue(sheetName, fmt.Sprintf("L%d", rowNum), 0)
			f.SetCellValue(sheetName, fmt.Sprintf("M%d", rowNum), "")
			f.SetCellValue(sheetName, fmt.Sprintf("N%d", rowNum), 0)
		}

		// Write party votes (starting from column O = 15)
		for i, partai := range partaiList {
			col := 15 + i
			cell, _ := excelize.CoordinatesToCellName(col, rowNum)
			if partaiVotes, ok := partaiData[kel.KelKode]; ok {
				if data, exists := partaiVotes[partai.NomorUrut]; exists {
					f.SetCellValue(sheetName, cell, data.JmlSuaraTotal)
				} else {
					f.SetCellValue(sheetName, cell, 0)
				}
			} else {
				f.SetCellValue(sheetName, cell, 0)
			}
		}

		rowNum++
	}

	// Apply borders to all cells
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if len(kelurahanList) > 0 {
		lastRow := len(kelurahanList) + 1
		lastCol, _ := excelize.ColumnNumberToName(12 + len(partaiList))
		f.SetCellStyle(sheetName, "A2", lastCol+strconv.Itoa(lastRow), dataStyle)
	}

	// Clean filename by removing invalid characters
	cleanKabNama := strings.ReplaceAll(kabNama, "/", "-")
	filename := fmt.Sprintf("DPRD_Kab_Partai_%s.xlsx", cleanKabNama)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadPilpresByProvince handles Excel download for Pilpres data per TPS by province
func (h *DPRDownloadHandler) DownloadPilpresByProvince(c echo.Context) error {
	proKode := c.Param("id")

	// Get province name
	var proNama string
	err := h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proKode).Scan(&proNama)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all TPS data with vote counts from hs_pilpres_tps
	tpsQuery := `
		SELECT
			t.tps_kode, t.tps_nama,
			t.kel_kode, t.kel_nama,
			t.kec_kode, t.kec_nama,
			t.kab_kode, t.kab_nama,
			t.dapil_kode, t.dapil_nama,
			t.chart, t.administrasi
		FROM hs_pilpres_tps t
		WHERE t.pro_kode = ?
		ORDER BY t.kab_nama, t.kec_nama, t.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, proKode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data Pilpres"
	f.SetSheetName("Sheet1", sheetName)

	// Set header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Set headers
	headers := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
		"ANIES-MUHAIMIN", "PRABOWO-GIBRAN", "GANJAR-MAHFUD",
		"PENGGUNA HAK PILIH", "SUARA SAH", "SUARA TIDAK SAH", "TOTAL SUARA",
	}

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 6)
	f.SetColWidth(sheetName, "B", "B", 25)
	f.SetColWidth(sheetName, "C", "C", 25)
	f.SetColWidth(sheetName, "D", "D", 25)
	f.SetColWidth(sheetName, "E", "E", 12)
	f.SetColWidth(sheetName, "F", "H", 18)
	f.SetColWidth(sheetName, "I", "M", 15)

	rowNum := 2
	for tpsRows.Next() {
		var tpsKode, tpsNama, kelKode, kelNama, kecKode, kecNama, kabKode, kabNama, dapilKode, dapilNama string
		var chartJSON, administrasiJSON sql.NullString

		if err := tpsRows.Scan(&tpsKode, &tpsNama, &kelKode, &kelNama, &kecKode, &kecNama, &kabKode, &kabNama, &dapilKode, &dapilNama, &chartJSON, &administrasiJSON); err != nil {
			continue
		}

		// Parse chart data (vote counts) - use pointers to distinguish between "no data" and "0"
		var chart map[string]interface{}
		var paslon1, paslon2, paslon3 *int
		if chartJSON.Valid && chartJSON.String != "" {
			if err := json.Unmarshal([]byte(chartJSON.String), &chart); err == nil {
				if val, ok := chart["100025"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						paslon1 = &temp
					}
				}
				if val, ok := chart["100026"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						paslon2 = &temp
					}
				}
				if val, ok := chart["100027"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						paslon3 = &temp
					}
				}
			}
		}

		// Parse administrasi data - use pointers to distinguish between "no data" and "0"
		var administrasi map[string]interface{}
		var dpt, penggunaHakPilih, suaraSah, suaraTidakSah, totalSuara *int
		if administrasiJSON.Valid && administrasiJSON.String != "" {
			if err := json.Unmarshal([]byte(administrasiJSON.String), &administrasi); err == nil {
				if val, ok := administrasi["pemilih_dpt_j"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						dpt = &temp
					}
				}
				if val, ok := administrasi["pengguna_total_j"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						penggunaHakPilih = &temp
					}
				}
				if val, ok := administrasi["suara_sah"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						suaraSah = &temp
					}
				}
				if val, ok := administrasi["suara_tidak_sah"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						suaraTidakSah = &temp
					}
				}
				if val, ok := administrasi["suara_total"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						totalSuara = &temp
					}
				}
			}
		}

		// Write data row
		colNum := 1
		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, rowNum-1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proNama)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilNama)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kelKode)
		colNum++

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tpsNama)
		colNum++

		// KODE TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tpsKode)
		colNum++

		// DPT - show "-" if no data, or the actual value (including 0)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if dpt != nil {
			f.SetCellValue(sheetName, cell, *dpt)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// ANIES-MUHAIMIN (Paslon 1)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if paslon1 != nil {
			f.SetCellValue(sheetName, cell, *paslon1)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// PRABOWO-GIBRAN (Paslon 2)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if paslon2 != nil {
			f.SetCellValue(sheetName, cell, *paslon2)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// GANJAR-MAHFUD (Paslon 3)
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if paslon3 != nil {
			f.SetCellValue(sheetName, cell, *paslon3)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// PENGGUNA HAK PILIH
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if penggunaHakPilih != nil {
			f.SetCellValue(sheetName, cell, *penggunaHakPilih)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// SUARA SAH
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if suaraSah != nil {
			f.SetCellValue(sheetName, cell, *suaraSah)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// SUARA TIDAK SAH
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if suaraTidakSah != nil {
			f.SetCellValue(sheetName, cell, *suaraTidakSah)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// TOTAL SUARA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if totalSuara != nil {
			f.SetCellValue(sheetName, cell, *totalSuara)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}

		rowNum++
	}

	// Apply borders to all data cells
	if rowNum > 2 {
		dataStyle, _ := f.NewStyle(&excelize.Style{
			Border: []excelize.Border{
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
				{Type: "right", Color: "000000", Style: 1},
			},
		})
		lastRow := rowNum - 1
		f.SetCellStyle(sheetName, "A2", fmt.Sprintf("M%d", lastRow), dataStyle)
	}

	// Set response headers
	cleanProNama := strings.ReplaceAll(proNama, "/", "-")
	filename := fmt.Sprintf("Pilpres_TPS_%s.xlsx", cleanProNama)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// DownloadDPDByProvinceTPS handles Excel download for DPD data per TPS by province
func (h *DPRDownloadHandler) DownloadDPDByProvinceTPS(c echo.Context) error {
	proKode := c.Param("id")

	// Get province name
	var proNama string
	err := h.db.QueryRow("SELECT pro_nama FROM pdpr_wil_pro WHERE pro_kode = ?", proKode).Scan(&proNama)
	if err != nil {
		return c.String(http.StatusNotFound, "Provinsi tidak ditemukan")
	}

	// Get all DPD candidates for this province
	calegQuery := `
		SELECT id, nama, nomor_urut
		FROM dpd_caleg
		WHERE pro_kode = ?
		ORDER BY nomor_urut
	`
	calegRows, err := h.db.Query(calegQuery, proKode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying DPD candidates")
	}
	defer calegRows.Close()

	type Caleg struct {
		ID         string
		Nama       string
		NomorUrut  int
	}

	var calegList []Caleg
	calegMap := make(map[string]Caleg)
	for calegRows.Next() {
		var c Caleg
		if err := calegRows.Scan(&c.ID, &c.Nama, &c.NomorUrut); err != nil {
			continue
		}
		calegList = append(calegList, c)
		calegMap[c.ID] = c
	}

	if len(calegList) == 0 {
		return c.String(http.StatusNotFound, "Tidak ada caleg DPD untuk provinsi ini")
	}

	// Get all TPS data with vote counts from hs_dpd_tps
	tpsQuery := `
		SELECT
			t.tps_kode, t.tps_nama,
			t.kel_kode, t.kel_nama,
			t.kec_kode, t.kec_nama,
			t.kab_kode, t.kab_nama,
			t.dapil_kode, t.dapil_nama,
			t.chart, t.administrasi
		FROM hs_dpd_tps t
		WHERE t.pro_kode = ?
		ORDER BY t.kab_nama, t.kec_nama, t.kel_nama, t.tps_nama
	`

	tpsRows, err := h.db.Query(tpsQuery, proKode)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error querying TPS data")
	}
	defer tpsRows.Close()

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Data DPD"
	f.SetSheetName("Sheet1", sheetName)

	// Set header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Set headers - first row: NO, location columns, DPT, then candidate names
	headers := []string{
		"NO", "PROVINSI", "KODE PROV", "DAPIL", "KODE DAPIL",
		"KAB/KOTA", "KODE KAB", "KECAMATAN", "KODE KEC",
		"KELURAHAN/DESA", "KODE DESA", "TPS", "KODE TPS", "DPT",
	}

	// Add candidate headers (Nomor Urut - Nama)
	for _, caleg := range calegList {
		headers = append(headers, fmt.Sprintf("%d - %s", caleg.NomorUrut, caleg.Nama))
	}

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 6)
	f.SetColWidth(sheetName, "B", "B", 25)
	f.SetColWidth(sheetName, "C", "C", 25)
	f.SetColWidth(sheetName, "D", "D", 25)
	f.SetColWidth(sheetName, "E", "E", 12)

	// Set candidate column widths
	startCol := 6
	endCol := startCol + len(calegList) - 1
	if endCol >= startCol {
		startColName, _ := excelize.ColumnNumberToName(startCol)
		endColName, _ := excelize.ColumnNumberToName(endCol)
		f.SetColWidth(sheetName, startColName, endColName, 20)
	}

	rowNum := 2
	for tpsRows.Next() {
		var tpsKode, tpsNama, kelKode, kelNama, kecKode, kecNama, kabKode, kabNama, dapilKode, dapilNama string
		var chartJSON, administrasiJSON sql.NullString

		if err := tpsRows.Scan(&tpsKode, &tpsNama, &kelKode, &kelNama, &kecKode, &kecNama, &kabKode, &kabNama, &dapilKode, &dapilNama, &chartJSON, &administrasiJSON); err != nil {
			continue
		}

		// Parse chart data (vote counts per candidate)
		var chart map[string]interface{}
		voteData := make(map[string]*int)

		if chartJSON.Valid && chartJSON.String != "" && chartJSON.String != "{\"null\":null}" {
			if err := json.Unmarshal([]byte(chartJSON.String), &chart); err == nil {
				for calegID := range calegMap {
					if val, ok := chart[calegID]; ok && val != nil {
						if v, ok := val.(float64); ok {
							temp := int(v)
							voteData[calegID] = &temp
						}
					}
				}
			}
		}

		// Parse administrasi data for DPT
		var administrasi map[string]interface{}
		var dpt *int
		if administrasiJSON.Valid && administrasiJSON.String != "" {
			if err := json.Unmarshal([]byte(administrasiJSON.String), &administrasi); err == nil {
				if val, ok := administrasi["pemilih_dpt_j"]; ok && val != nil {
					if v, ok := val.(float64); ok {
						temp := int(v)
						dpt = &temp
					}
				}
			}
		}

		// Write data row
		colNum := 1
		// NO
		cell, _ := excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, rowNum-1)
		colNum++

		// PROVINSI
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proNama)
		colNum++

		// KODE PROV
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, proKode)
		colNum++

		// DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilNama)
		colNum++

		// KODE DAPIL
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, dapilKode)
		colNum++

		// KAB/KOTA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kabNama)
		colNum++

		// KODE KAB
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kabKode)
		colNum++

		// KECAMATAN
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kecNama)
		colNum++

		// KODE KEC
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kecKode)
		colNum++

		// KELURAHAN/DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kelNama)
		colNum++

		// KODE DESA
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, kelKode)
		colNum++

		// TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tpsNama)
		colNum++

		// KODE TPS
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		f.SetCellValue(sheetName, cell, tpsKode)
		colNum++

		// DPT
		cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
		if dpt != nil {
			f.SetCellValue(sheetName, cell, *dpt)
		} else {
			f.SetCellStr(sheetName, cell, "-")
		}
		colNum++

		// Write vote counts for each candidate
		for _, caleg := range calegList {
			cell, _ = excelize.CoordinatesToCellName(colNum, rowNum)
			if vote, ok := voteData[caleg.ID]; ok && vote != nil {
				f.SetCellValue(sheetName, cell, *vote)
			} else {
				f.SetCellStr(sheetName, cell, "-")
			}
			colNum++
		}

		rowNum++
	}

	// Apply borders to all data cells
	if rowNum > 2 {
		dataStyle, _ := f.NewStyle(&excelize.Style{
			Border: []excelize.Border{
				{Type: "left", Color: "000000", Style: 1},
				{Type: "top", Color: "000000", Style: 1},
				{Type: "bottom", Color: "000000", Style: 1},
				{Type: "right", Color: "000000", Style: 1},
			},
		})
		lastRow := rowNum - 1
		lastCol := getColumnName(5 + len(calegList))
		f.SetCellStyle(sheetName, "A2", fmt.Sprintf("%s%d", lastCol, lastRow), dataStyle)
	}

	// Set response headers
	cleanProNama := strings.ReplaceAll(proNama, "/", "-")
	filename := fmt.Sprintf("DPD_TPS_%s.xlsx", cleanProNama)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

// Helper function to get column name from column number (1-based)
func getColumnName(colNum int) string {
	colName, _ := excelize.ColumnNumberToName(colNum)
	return colName
}
