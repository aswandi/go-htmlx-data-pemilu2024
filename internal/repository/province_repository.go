package repository

import (
	"database/sql"
	"datapemilu2024/internal/models"
)

type ProvinceRepository struct {
	db *sql.DB
}

func NewProvinceRepository(db *sql.DB) *ProvinceRepository {
	return &ProvinceRepository{db: db}
}

func (r *ProvinceRepository) GetAllProvinces() ([]models.Province, error) {
	query := `
		SELECT
			p.id,
			p.pro_kode AS code,
			p.pro_nama AS name,
			COALESCE(p.jumlah_tps, 0) AS jumlah_tps,
			COALESCE(p.total_dpt, 0) AS jumlah_dpt,
			(SELECT COUNT(*)
			 FROM dpr_ri_dapil d
			 WHERE SUBSTRING(d.dapil_kode, 1, 2) = p.pro_kode) AS jumlah_dapil,
			(SELECT COUNT(*)
			 FROM pdpr_wil_kab k
			 WHERE SUBSTRING(k.kab_kode, 1, 2) = p.pro_kode AND k.tingkat = 2) AS jumlah_kab_kota
		FROM pdpr_wil_pro p
		WHERE p.tingkat = 1
		ORDER BY p.pro_kode
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var provinces []models.Province
	for rows.Next() {
		var prov models.Province
		err := rows.Scan(
			&prov.ID,
			&prov.Code,
			&prov.Name,
			&prov.JumlahTPS,
			&prov.JumlahDPT,
			&prov.JumlahDapil,
			&prov.JumlahKabKota,
		)
		if err != nil {
			return nil, err
		}
		provinces = append(provinces, prov)
	}

	return provinces, nil
}
