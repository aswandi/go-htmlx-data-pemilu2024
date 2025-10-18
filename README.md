# Database Pemilu 2024

A web application for managing and displaying Indonesian election data (Pemilu 2024) across all administrative levels.

## Tech Stack

- Go
- Templ
- Tailwind CSS
- HTMX

## Database Configuration

- **Database**: MySQL
- **Host**: 127.0.0.1:3306
- **Database Name**: pileg2024
- **Username**: root

## Data Structure & Architecture

### Administrative Hierarchy Tables
The database contains complete Indonesian administrative divisions with election data:

- `pdpr_wil_pro` - Province level
- `pdpr_wil_kab` - Regency/City level (Kabupaten/Kota)
- `pdpr_wil_kec` - District level (Kecamatan)
- `pdpr_wil_kel` - Village level (Kelurahan)
- `pdpr_wil_dapil` - Electoral districts (Dapil)
- `pdpr_wil_tps` - Polling stations (TPS)

### Election Types & Levels

1. **Presidential Election (Pilpres/PPWP)**: National → Province → Regency/City → District → Village → TPS
2. **DPD (Regional Representatives)**: Provincial level, 4 representatives per province, voting down to TPS
3. **DPR RI (National Parliament)**: Provincial dapil level, varies by province (1+ dapil per province)
4. **DPRD Province**: Provincial dapil level, districts divided by regency boundaries
5. **DPRD Regency/City**: Regency dapil level, districts divided by sub-district boundaries

### Candidate Data Tables
- `dpd_caleg` - DPD candidates
- `dpr_ri_caleg` - DPR RI candidates
- `dprd_pro_caleg` - Provincial DPRD candidates
- `dprd_kab_caleg` - Regency/City DPRD candidates

### Table Prefix Conventions
- `hs_*` - Vote counting (hitung suara)
- `hr_*` - Vote recapitulation (hitung rekapitulasi)
- `pdpr_*` - DPR RI election
- `pdprdp_*` - Provincial DPRD election
- `pdprdk_*` - Aceh special DPRDK election only
- `dprd_kab_*` - Regency/City DPRD
- `pilpres_*` / `ppwp_*` - Presidential election
- `*_wil_*` - Regional/administrative data
- `*_nas` - National level
- `*_pro` - Province level
- `*_kab` - Regency level
- `*_kec` - District level
- `*_kel` - Village level
- `*_dapil` - Electoral district
- `*_tps` - Polling station

## Development Commands

```bash
# Initialize Go module (if not done)
go mod init datapemilu2024

# Install dependencies
go get github.com/a-h/templ
go get github.com/go-sql-driver/mysql
# ... other dependencies as needed

# Generate templ files
templ generate

# Build Tailwind CSS
npx tailwindcss -i ./input.css -o ./static/output.css --watch

# Run application
go run main.go

# Build for production
go build -o datapemilu2024
```

## Changelog

```
DATABASE PEMILU 2024

Sistem Informasi Pemilihan Umum Indonesia


CHANGELOG

Riwayat perubahan dan pembaruan aplikasi Database Pemilu 2024


VERSION 1.10.0

18 Oktober 2025

🐛 PERBAIKAN CRITICAL BUG

 * Fix Download DPR RI Partai per Dapil:
   * Memperbaiki masalah data TPS, DPT, dan suara partai yang menjadi 0 atau kosong
   * Root cause: Query menggunakan kab_kode hanya mengambil data kabupaten pertama dalam dapil
   * Solusi: Mengubah query untuk menggunakan dapil_id agar mencakup seluruh kabupaten dalam dapil

🔧 PERUBAHAN TEKNIS

 * File: internal/handlers/dpr_download_handler.go
 * Fungsi: DownloadDPRRIPartai()
 * Baris 400: Query TPS menggunakan WHERE dapil_id = ? (sebelumnya WHERE kab_kode = ?)
 * Baris 425: Query vote data menggunakan WHERE dapil_id = ? (sebelumnya WHERE kab_kode = ?)

✅ HASIL PERBAIKAN

 * Data TPS per kelurahan kini terisi dengan benar
 * Data DPT (Daftar Pemilih Tetap) kini akurat
 * Suara partai untuk semua kabupaten dalam dapil kini lengkap
 * Endpoint: /download/dapil/dpr-ri-partai/:code

📊 CONTOH KASUS

 * Dapil 1101 (ACEH I) sebelumnya menampilkan TPS dan DPT = 0
 * Setelah perbaikan: 8,478 TPS dengan data lengkap di semua 3,648 kelurahan
 * File Excel berukuran 423KB dengan 3,649 baris data


VERSION 1.9.0

18 Oktober 2025

🔧 PERBAIKAN FORMAT EXCEL

 * Standardisasi Kolom Download Data TPS:
   * Semua download data TPS level provinsi kini memiliki format kolom yang konsisten
   * Format standar: NO, PROVINSI, KODE PROV, DAPIL, KODE DAPIL, KAB/KOTA, KODE KAB, KECAMATAN, KODE KEC, KELURAHAN/DESA, KODE DESA, TPS, KODE TPS, DPT
   * Berlaku untuk: Pilpres, DPD TPS, DPR RI Caleg TPS
 * Perbaikan Download Data Perdesa (DPD):
   * Update query untuk join dengan tabel pdpr_wil_kel dan pdpr_wil_dapil
   * Menambahkan kolom DAPIL dan KODE DAPIL dari relasi tabel DPR RI
   * Menambahkan kolom TPS dan KODE TPS untuk detail data
   * Format: NO, PROVINSI, KODE PROV, DAPIL, KODE DAPIL, KAB/KOTA, KODE KAB, KECAMATAN, KODE KEC, KELURAHAN/DESA, KODE DESA, TPS, KODE TPS, DPT

📊 PENINGKATAN DATA

 * Parsing DPT dari field administrasi JSON (pemilih_dpt_j)
 * Data dapil diambil dari relasi dengan tabel DPR RI untuk akurasi
 * Handling kosong/null untuk kolom dapil dan TPS

🎯 ENDPOINT YANG DIPERBAIKI

 * /download/provinsi/pilpres/:id - Format kolom distandarisasi
 * /download/provinsi/dpd-tps/:id - Format kolom distandardisasi
 * /download/provinsi/dpr-ri-caleg-tps/:code - Format kolom distandardisasi
 * /download/provinsi/dpd/:code - Menambahkan kolom dapil dan TPS


VERSION 1.8.0

17 Oktober 2025

✨ FITUR BARU

 * Download DPRD Kabupaten Caleg per Provinsi (Multi-Sheet):
   * Endpoint /download/dprd-kab-caleg/:id
   * 1 file Excel per provinsi dengan sheet terpisah untuk setiap dapil
   * Data caleg per kecamatan dalam setiap dapil
   * Header: NO, Provinsi, Kode Prov, Dapil, Kode Dapil, Kab/Kota, Kode Kab, Kecamatan, Kode Kec
   * Kolom dinamis untuk setiap caleg dengan format: Nomor Urut. Nama Caleg
   * Kolom TOTAL untuk jumlah suara total per kecamatan
 * Download DPD per TPS:
   * Endpoint /download/provinsi/dpd-tps/:id
   * Data suara DPD detail sampai level TPS per provinsi
   * Format Excel dengan kolom lengkap termasuk data per TPS
 * Perbaikan Endpoint Pilpres:
   * URL diubah dari /download/provinsi/pilpres/:code
   * Menjadi /download/provinsi/pilpres/:id
   * Handler baru DownloadPilpresByProvince

🔧 PENINGKATAN TEKNIS

 * Multi-sheet Excel generation untuk data kompleks
 * Fungsi sanitizeSheetName() untuk nama sheet yang valid (max 31 karakter)
 * Query optimasi untuk agregasi data per dapil dan kecamatan
 * Menggunakan tabel hr_dprd_kab_kec untuk data DPRD Kabupaten
 * Style Excel konsisten: header biru dengan border dan text wrapping

📊 STRUKTUR DATA

 * Setiap sheet mewakili 1 dapil DPRD Kabupaten
 * Data diorganisir per kecamatan dalam dapil
 * Caleg diurutkan berdasarkan nomor urut
 * Kolom lebar otomatis untuk kemudahan membaca


VERSION 1.7.0

16 Oktober 2025

✨ FITUR BARU

 * Download Excel Data Suara DPR RI per Dapil:
   * Download suara partai per TPS - Endpoint /download/dapil/dpr-ri-partai-tps/:code
   * Download suara caleg per TPS - Endpoint /download/dapil/dpr-ri-caleg-tps/:code
   * Data mencakup semua TPS di semua kabupaten dalam 1 dapil
   * Format Excel dengan kolom lengkap: Provinsi, Dapil, Kabupaten, Kecamatan, Kelurahan, TPS + data suara

🔧 PENINGKATAN TEKNIS

 * Menggunakan data dari tabel hr_dpr_ri_kel untuk akurasi tinggi
 * Parsing JSON data suara per TPS dengan handling error yang lebih baik
 * Agregasi suara caleg menjadi suara partai
 * Format header Excel untuk caleg: Partai Singkat + Nomor Urut + Nama Caleg
 * Style Excel dengan header biru dan border pada semua cell

📊 FORMAT DATA

 * Partai per TPS: Kolom per partai + Total suara
 * Caleg per TPS: Kolom per caleg + Total suara
 * Penanganan nilai "-" untuk TPS tanpa data
 * Penanganan nilai "0" untuk TPS dengan data tapi tidak ada suara


VERSION 1.6.0

14 Oktober 2025

🚀 OPTIMASI PERFORMA DATABASE

 * Perbaikan critical: CPU usage dari 181% menjadi ~10% (pengurangan 95%)
 * Eliminasi semua slow queries (1000-2000 detik → <1 detik)
 * System load turun drastis dari 10.90 menjadi 0.76

🔧 DATABASE OPTIMIZATION

 * Penambahan 40+ strategic indexes di seluruh tabel:
   * pdpr_wil_tps: 12 index baru (composite + single column)
   * pdpr_wil_pro, kab, kec, kel, dapil: Index pada kolom kode & ID
   * Candidate tables: Index untuk JOIN yang lebih cepat
 * Total 27 indexes pada tabel pdpr_wil_tps (880K+ rows)
 * Composite indexes untuk optimasi query hierarchical

⚙️ MYSQL CONFIGURATION TUNING

 * InnoDB Buffer Pool: 4GB (50% dari 8GB RAM)
 * Query execution timeout: 30 detik maksimal
 * Connection timeout settings
 * Memory optimization:
   * tmp_table_size: 256MB
   * max_heap_table_size: 256MB
   * sort_buffer_size: 4MB
   * join_buffer_size: 4MB
 * Table cache: 4000 tables, 2000 definitions
 * Max connections: 200

📊 PERFORMANCE IMPACT

 * Query performance: 1000-2000s → <1s (peningkatan 1000x+)
 * CPU usage turun 95%
 * Sistem lebih stabil dan responsif
 * Memory management optimal untuk 8GB RAM

📝 DOKUMENTASI

 * OPTIMIZATION_REPORT.md - Laporan lengkap optimasi database
 * CHANGELOG.md - Dokumentasi perubahan versi
 * optimize_indexes_final.sql - Script SQL untuk index optimization


VERSION 1.5.0

13 Oktober 2024

✨ FITUR BARU

 * Halaman Dapil (Daerah Pemilihan) dengan 2 kolom baru:
   * Data Perdesa - Download data per kelurahan/desa
   * DATA SUARA TPS - Download data per TPS
 * Download data per Dapil untuk berbagai jenis pemilu:
   * PILPRES - Data pilpres per kelurahan (Excel format)
   * DPR RI PARTAI - Data suara partai per kelurahan dan per TPS
   * DPR RI CALEG - Data suara caleg per kelurahan dan per TPS
 * Klik pada jumlah dapil di halaman provinsi untuk melihat detail dapil

🔧 PERBAIKAN

 * Query database menggunakan tabel yang benar (pdpr_wil_dapil)
 * Parsing JSON data pilpres dari tabel hs_pilpres_kel
 * Format data paslon pilpres (Paslon 1, 2, 3)
 * Handler download dapil untuk semua jenis pemilu

⚠️ CATATAN PENTING

 * Download Data TPS: Untuk dapil dengan jumlah TPS sangat banyak (15.000+ TPS), download data per TPS mungkin mengalami timeout karena volume data yang sangat besar
 * Rekomendasi: Gunakan download "Data Perdesa" (per kelurahan) untuk performa lebih baik pada dapil besar
 * Status: PILPRES per TPS belum tersedia (dalam pengembangan)

🎨 PENINGKATAN UI/UX

 * Tabel dapil dengan gradient indigo-purple
 * Tombol download tersusun rapi dengan color-coded buttons
 * Status visual untuk fitur yang belum tersedia (abu-abu)
 * Link kembali ke daftar provinsi


VERSION 1.4.0

12 Oktober 2024

🔧 PERBAIKAN

 * Format header Excel untuk download DPR RI Caleg per TPS:
   * Baris 1: Nama Partai Singkat
   * Baris 2: Nomor Urut Caleg
   * Baris 3: Nama Caleg
 * Logika penampilan data suara:
   * Tampilkan "-" jika tidak ada data
   * Tampilkan "0" jika ada data dengan nilai 0
 * Perbaikan error 500 pada download kabupaten:
   * Download DPR RI Caleg per TPS kabupaten
   * Download DPR RI Partai per TPS kabupaten

🎨 PENINGKATAN UI/UX

 * Header Excel lebih rapi dan mudah dibaca
 * Konsistensi format data di semua level download


VERSION 1.3.0

11 Oktober 2024

✨ FITUR BARU

 * Footer dengan copyright aswandi.or.id
 * Halaman changelog untuk dokumentasi perubahan
 * Link changelog pada footer


VERSION 1.2.0

10 Oktober 2024

✨ FITUR BARU

 * Halaman detail Kabupaten/Kota per provinsi
 * Data jumlah kecamatan dan kelurahan
 * Download data per kabupaten untuk semua jenis pemilu:
   * PILPRES (Presidential Election)
   * DPD (Regional Representatives)
   * DPR RI - Partai & Caleg
   * DPRD Provinsi - Partai & Caleg
   * DPRD Kabupaten/Kota - Partai & Caleg
 * Navigasi breadcrumb untuk kembali ke halaman provinsi

🎨 PENINGKATAN UI/UX

 * Warna gradient hijau-teal untuk tabel kabupaten
 * Tombol download dengan berbagai warna sesuai jenis data
 * Hover effects pada baris tabel
 * Info box dengan total kabupaten/kota


VERSION 1.1.0

10 Oktober 2024

✨ FITUR BARU

 * Fitur download data DPR RI per provinsi:
   * Download DPR RI Partai (Excel format)
   * Download DPR RI Caleg (Excel format)
 * Tombol download interaktif pada setiap baris provinsi
 * Status visual untuk fitur yang belum tersedia

🛠️ TEKNIS

 * Implementasi Excel generation dengan library excelize
 * API endpoints untuk download data provinsi
 * Handler khusus untuk DPR RI download


VERSION 1.0.0

10 Oktober 2024

🎉 RILIS AWAL

 * Halaman utama dengan tabel data provinsi Indonesia
 * Tampilan informasi lengkap per provinsi:
   * Jumlah Daerah Pemilihan (Dapil)
   * Jumlah Kabupaten/Kota
   * Jumlah TPS (Tempat Pemungutan Suara)
   * Jumlah DPT (Daftar Pemilih Tetap)
 * Link interaktif pada nama provinsi

🎨 DESAIN & UI

 * Modern responsive design dengan Tailwind CSS
 * Gradient background (blue-purple)
 * Animasi slide-in pada load
 * Card hover effects
 * Color-coded badges untuk statistik
 * Gradient header pada tabel (blue-purple)
 * Navigation bar dengan gradient logo

⚙️ INFRASTRUKTUR

 * Tech stack: Go + Templ + HTMX + Tailwind CSS
 * Echo framework untuk web server
 * MySQL database connection
 * Repository pattern untuk data access
 * Nginx sebagai reverse proxy

TENTANG APLIKASI

Database Pemilu 2024 adalah aplikasi web untuk mengelola dan menampilkan data pemilihan umum Indonesia 2024 di semua tingkat administratif.

Dikembangkan oleh aswandi.or.id

© 2024 Database Pemilu Indonesia. All rights reserved.
Developed by aswandi.or.id

Changelog
```