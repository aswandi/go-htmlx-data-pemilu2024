package repository

import (
	"database/sql"
	"datapemilu2024/internal/models"
)

type DapilRepository struct {
	db *sql.DB
}

func NewDapilRepository(db *sql.DB) *DapilRepository {
	return &DapilRepository{db: db}
}

func (r *DapilRepository) GetDapilByProvinceCode(provinceCode string) ([]models.Dapil, error) {
	query := `
		SELECT
			d.dapil_id,
			d.dapil_kode AS code,
			d.dapil_nama AS name,
			SUBSTRING(d.dapil_kode, 1, 2) AS province_code,
			(SELECT p.pro_nama FROM pdpr_wil_pro p WHERE p.pro_kode = SUBSTRING(d.dapil_kode, 1, 2)) AS province_name,
			COALESCE(d.jml_kursi, 0) AS jumlah_kursi,
			(SELECT COUNT(*) FROM dpr_ri_caleg c WHERE c.dapil_kode = d.dapil_kode) AS jumlah_caleg,
			(SELECT COUNT(DISTINCT c.partai_id) FROM dpr_ri_caleg c WHERE c.dapil_kode = d.dapil_kode) AS jumlah_partai
		FROM dpr_ri_dapil d
		WHERE SUBSTRING(d.dapil_kode, 1, 2) = ?
		ORDER BY d.dapil_kode
	`

	rows, err := r.db.Query(query, provinceCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dapils []models.Dapil
	for rows.Next() {
		var dapil models.Dapil
		err := rows.Scan(
			&dapil.ID,
			&dapil.Code,
			&dapil.Name,
			&dapil.ProvinceCode,
			&dapil.ProvinceName,
			&dapil.JumlahKursi,
			&dapil.JumlahCaleg,
			&dapil.JumlahPartai,
		)
		if err != nil {
			return nil, err
		}
		dapils = append(dapils, dapil)
	}

	return dapils, nil
}
