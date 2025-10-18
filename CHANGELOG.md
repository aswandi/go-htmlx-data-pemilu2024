# Changelog

All notable changes to the Database Pemilu 2024 project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project setup with Go + Templ + HTMX + Tailwind CSS
- MySQL database configuration for pileg2024
- Comprehensive database index optimization (40+ indexes)
- MySQL performance tuning for 8GB RAM systems
- OPTIMIZATION_REPORT.md with detailed performance metrics
- **Download Excel Data Suara DPR RI per Dapil**:
  - Download suara partai per TPS dalam 1 dapil (`/download/dapil/dpr-ri-partai-tps/:code`)
  - Download suara caleg per TPS dalam 1 dapil (`/download/dapil/dpr-ri-caleg-tps/:code`)
  - Format Excel dengan kolom lengkap: Provinsi, Dapil, Kabupaten, Kecamatan, Kelurahan, TPS + data suara
  - Menggunakan data dari tabel `hr_dpr_ri_kel` untuk akurasi tinggi

### Changed
- MySQL configuration in `/etc/mysql/mysql.conf.d/mysqld.cnf`:
  - Added InnoDB buffer pool size: 4GB
  - Added query execution timeout: 30 seconds
  - Added connection timeout settings
  - Added performance tuning parameters

### Fixed
- Critical performance issue: CPU usage reduced from 181% to ~10%
- Eliminated slow queries (1000-2000 seconds execution time)
- System load reduced from 10.90 to 0.76

### Performance
- **Database Indexes**: Added 40+ indexes across all tables
  - `pdpr_wil_tps`: 12 new indexes (composite + single column)
  - `pdpr_wil_pro`, `kab`, `kec`, `kel`, `dapil`: Indexes on code & ID columns
  - All candidate tables: Optimized indexes for faster JOINs
- **Query Performance**: Improved from 1000-2000s to <1s (estimated)
- **CPU Usage**: Reduced by 95% (from 181% to ~10%)
- **Memory**: InnoDB buffer pool optimized at 4GB

## [0.1.0] - 2025-10-14

### Added
- Initial commit with database structure
- Province data table from `pdpr_wil_pro`
- Administrative hierarchy tables setup:
  - `pdpr_wil_pro` (Province)
  - `pdpr_wil_kab` (Regency/City)
  - `pdpr_wil_kec` (District)
  - `pdpr_wil_kel` (Village)
  - `pdpr_wil_dapil` (Electoral District)
  - `pdpr_wil_tps` (Polling Station)
- Candidate tables:
  - `dpd_caleg` (DPD candidates)
  - `dpr_ri_caleg` (DPR RI candidates)
  - `dprd_pro_caleg` (Provincial DPRD candidates)
  - `dprd_kab_caleg` (Regency/City DPRD candidates)

### Infrastructure
- Nginx web server configuration
- MySQL 8.0+ database server
- Domain: datapemilu2024.aplikasiweb.my.id
- Document root: `/var/www/html/datapemilu2024`

---

## Release Notes

### v0.1.0 - Initial Release (2025-10-14)

First release of Database Pemilu 2024 application with:
- Complete Indonesian election data structure
- 880K+ polling stations (TPS)
- Province, regency, district, and village level data
- All candidate data for DPD, DPR RI, DPRD Province, and DPRD Regency
- Optimized database performance with comprehensive indexing

### Performance Optimization (2025-10-14)

Critical database optimization applied:
- Fixed CPU overload issue (181% → 10%)
- Added 40+ strategic indexes
- Configured MySQL for optimal performance
- Eliminated all slow queries

---

## Development Commands

```bash
# Generate templ files
templ generate

# Build Tailwind CSS
npx tailwindcss -i ./input.css -o ./static/output.css --watch

# Run application
go run main.go

# Build for production
go build -o datapemilu2024
```

## Database Maintenance

```bash
# Analyze tables (run monthly)
mysql -u root -p pileg2024 -e "ANALYZE TABLE pdpr_wil_tps, pdpr_wil_pro, pdpr_wil_kab;"

# Check slow queries
mysql -u root -p -e "SELECT * FROM information_schema.processlist WHERE time > 5;"

# Monitor buffer pool
mysql -u root -p -e "SHOW STATUS LIKE 'Innodb_buffer_pool_%';"
```
