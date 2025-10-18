-- ============================================
-- MySQL Index Optimization for Pemilu 2024
-- ============================================

-- 1. Optimize pdpr_wil_tps (Polling Stations)
-- This is the largest table with 880K+ rows
ALTER TABLE pdpr_wil_tps
  ADD INDEX idx_pro_kode (pro_kode),
  ADD INDEX idx_dapil_kode (dapil_kode),
  ADD INDEX idx_kab_kode (kab_kode),
  ADD INDEX idx_kec_kode (kec_kode),
  ADD INDEX idx_tps_kode (tps_kode),
  ADD INDEX idx_tps_id (tps_id),
  ADD INDEX idx_dapil_id (dapil_id),
  -- Composite indexes for common JOIN patterns
  ADD INDEX idx_pro_kab (pro_id, kab_id),
  ADD INDEX idx_kab_kec (kab_id, kec_id),
  ADD INDEX idx_kec_kel (kec_id, kel_id),
  ADD INDEX idx_pro_kab_kec (pro_id, kab_id, kec_id),
  ADD INDEX idx_kab_kec_kel (kab_id, kec_id, kel_id);

-- 2. Optimize pdpr_wil_pro (Provinces)
ALTER TABLE pdpr_wil_pro
  ADD INDEX idx_pro_id (pro_id),
  ADD INDEX idx_pro_kode (pro_kode);

-- 3. Optimize pdpr_wil_kab (Regencies)
ALTER TABLE pdpr_wil_kab
  ADD INDEX idx_kab_id (kab_id),
  ADD INDEX idx_kab_kode (kab_kode),
  ADD INDEX idx_pro_kab_composite (pro_id, kab_id);

-- 4. Optimize pdpr_wil_kec (Districts)
ALTER TABLE pdpr_wil_kec
  ADD INDEX idx_kec_id (kec_id),
  ADD INDEX idx_kec_kode (kec_kode),
  ADD INDEX idx_pro_id_kec (pro_id),
  ADD INDEX idx_dapil_id_kec (dapil_id),
  ADD INDEX idx_pro_kab_kec_composite (pro_id, kab_id, kec_id);

-- 5. Optimize pdpr_wil_kel (Villages)
ALTER TABLE pdpr_wil_kel
  ADD INDEX idx_kel_kode (kel_kode),
  ADD INDEX idx_pro_id_kel (pro_id),
  ADD INDEX idx_dapil_id_kel (dapil_id),
  ADD INDEX idx_pro_kab_kec_kel_composite (pro_id, kab_id, kec_id, kel_id);

-- 6. Optimize pdpr_wil_dapil (Electoral Districts)
ALTER TABLE pdpr_wil_dapil
  ADD INDEX idx_dapil_kode (dapil_kode),
  ADD INDEX idx_dapil_id_dapil (dapil_id),
  ADD INDEX idx_pro_dapil_composite (pro_id, dapil_id);

-- 7. Drop duplicate/redundant indexes on pdpr_wil_tps
-- These are redundant with the new composite indexes
ALTER TABLE pdpr_wil_tps
  DROP INDEX namaPro,
  DROP INDEX namaKab,
  DROP INDEX namaKec,
  DROP INDEX namaKel,
  DROP INDEX tps_nama;

-- 8. Optimize DPRD Province tables (if exist)
ALTER TABLE pdprdp_wil_tps
  ADD INDEX idx_pro_kode (pro_kode),
  ADD INDEX idx_kab_kode (kab_kode),
  ADD INDEX idx_kec_kode (kec_kode),
  ADD INDEX idx_kel_kode (kel_kode),
  ADD INDEX idx_pro_id (pro_id),
  ADD INDEX idx_kab_id (kab_id),
  ADD INDEX idx_kec_id (kec_id),
  ADD INDEX idx_kel_id (kel_id);

-- 9. Optimize DPRDK Aceh tables (if exist)
ALTER TABLE pdprdk_wil_tps
  ADD INDEX idx_pro_kode (pro_kode),
  ADD INDEX idx_kab_kode (kab_kode),
  ADD INDEX idx_kec_kode (kec_kode),
  ADD INDEX idx_kel_kode (kel_kode),
  ADD INDEX idx_pro_id (pro_id),
  ADD INDEX idx_kab_id (kab_id),
  ADD INDEX idx_kec_id (kec_id),
  ADD INDEX idx_kel_id (kel_id);

-- 10. Optimize Candidate tables
ALTER TABLE dpr_ri_caleg
  ADD INDEX idx_dapil_kode (dapil_kode),
  ADD INDEX idx_partai_id (partai_id);

ALTER TABLE dprd_pro_caleg
  ADD INDEX idx_pro_id (pro_id),
  ADD INDEX idx_pro_kode (pro_kode),
  ADD INDEX idx_dapil_id (dapil_id),
  ADD INDEX idx_dapil_kode (dapil_kode),
  ADD INDEX idx_partai_id (partai_id),
  ADD INDEX idx_pro_dapil (pro_id, dapil_id);

ALTER TABLE dprd_kab_caleg
  ADD INDEX idx_pro_id (pro_id),
  ADD INDEX idx_kab_id (kab_id),
  ADD INDEX idx_kab_kode (kab_kode),
  ADD INDEX idx_dapil_id (dapil_id),
  ADD INDEX idx_dapil_kode (dapil_kode),
  ADD INDEX idx_partai_id (partai_id),
  ADD INDEX idx_kab_dapil (kab_id, dapil_id);

ALTER TABLE dpd_caleg
  ADD INDEX idx_pro_kode (pro_kode),
  ADD INDEX idx_nomor_urut (nomor_urut);

-- ============================================
-- MySQL Configuration Tuning
-- ============================================

-- These settings improve query performance
SET GLOBAL innodb_buffer_pool_size = 2147483648; -- 2GB
SET GLOBAL key_buffer_size = 268435456; -- 256MB
SET GLOBAL query_cache_size = 67108864; -- 64MB (if using MySQL < 8.0)
SET GLOBAL tmp_table_size = 134217728; -- 128MB
SET GLOBAL max_heap_table_size = 134217728; -- 128MB
SET GLOBAL sort_buffer_size = 4194304; -- 4MB
SET GLOBAL read_buffer_size = 2097152; -- 2MB
SET GLOBAL join_buffer_size = 4194304; -- 4MB

-- ============================================
-- Analyze tables after index creation
-- ============================================

ANALYZE TABLE pdpr_wil_tps;
ANALYZE TABLE pdpr_wil_pro;
ANALYZE TABLE pdpr_wil_kab;
ANALYZE TABLE pdpr_wil_kec;
ANALYZE TABLE pdpr_wil_kel;
ANALYZE TABLE pdpr_wil_dapil;
ANALYZE TABLE pdprdp_wil_tps;
ANALYZE TABLE pdprdk_wil_tps;
ANALYZE TABLE dpr_ri_caleg;
ANALYZE TABLE dprd_pro_caleg;
ANALYZE TABLE dprd_kab_caleg;
ANALYZE TABLE dpd_caleg;
