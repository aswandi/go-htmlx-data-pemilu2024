package repository

import (
	"database/sql"
	"datapemilu2024/internal/models"
)

type KabupatenRepository struct {
	db *sql.DB
}

func NewKabupatenRepository(db *sql.DB) *KabupatenRepository {
	return &KabupatenRepository{db: db}
}

func (r *KabupatenRepository) GetKabupatenByProvinceCode(provinceCode string) ([]models.Kabupaten, error) {
	query := `
		SELECT
			k.id,
			k.kab_kode AS code,
			k.kab_nama AS name,
			k.pro_kode AS province_code,
			k.pro_nama AS province_name,
			COALESCE(k.jumlah_kecamatan, 0) AS jumlah_kecamatan,
			COALESCE(k.jumlah_kelurahan, 0) AS jumlah_kelurahan
		FROM pdpr_wil_kab k
		WHERE k.pro_kode = ? AND k.tingkat = 2
		ORDER BY k.kab_kode
	`

	rows, err := r.db.Query(query, provinceCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kabupatens []models.Kabupaten
	for rows.Next() {
		var kab models.Kabupaten
		err := rows.Scan(
			&kab.ID,
			&kab.Code,
			&kab.Name,
			&kab.ProvinceCode,
			&kab.ProvinceName,
			&kab.JumlahKecamatan,
			&kab.JumlahKelurahan,
		)
		if err != nil {
			return nil, err
		}
		kabupatens = append(kabupatens, kab)
	}

	return kabupatens, nil
}
