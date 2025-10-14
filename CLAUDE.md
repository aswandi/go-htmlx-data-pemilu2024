# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Database Pemilu 2024** - A web application for managing and displaying Indonesian election data (Pemilu 2024) across all administrative levels.

**Tech Stack**: Go + Templ + Tailwind CSS + HTMX

**Domain**: datapemilu2024.aplikasiweb.my.id
**Document Root**: `/var/www/html/datapemilu2024`
**Web Server**: Nginx (running on port 80)

## Database Configuration

**Database**: MySQL
**Host**: 127.0.0.1:3306
**Database Name**: pileg2024
**Username**: root
**Password**: StrongPassword123!

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

## UI Requirements

- Modern, bright, responsive design
- Tailwind CSS for styling
- HTMX for dynamic interactions
- Animations on cards, tables, and sections
- Mobile-responsive layout

## Initial Implementation

First page to implement: **Province Data Table** from `pdpr_wil_pro`

Columns required:
1. NO (Number)
2. NAMA PROVINSI (Province Name)
3. JUMLAH DAPIL (Number of Electoral Districts) - from `pdpr_wil_dapil`
4. JUMLAH KABUPATEN/KOTA (Number of Regencies/Cities) - from `pdpr_wil_kab`
5. JUMLAH TPS (Number of Polling Stations) - from `pdpr_wil_tps`
6. JUMLAH DPT (Number of Registered Voters)

## Development Commands

*Note: Project structure not yet initialized. When setting up, typical Go commands will include:*

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

## Nginx Configuration

Virtual host: `/etc/nginx/sites-available/datapemilu2024.aplikasiweb.my.id`
Document root points to this directory with PHP-FPM support and static asset caching.
