package models

type Kabupaten struct {
	ID              int
	Code            string
	Name            string
	ProvinceCode    string
	ProvinceName    string
	JumlahKecamatan int
	JumlahKelurahan int
}
