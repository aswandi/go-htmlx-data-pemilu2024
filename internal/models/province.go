package models

type Province struct {
	ID             int    `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	JumlahDapil    int    `json:"jumlah_dapil"`
	JumlahKabKota  int    `json:"jumlah_kab_kota"`
	JumlahTPS      int    `json:"jumlah_tps"`
	JumlahDPT      int    `json:"jumlah_dpt"`
}
