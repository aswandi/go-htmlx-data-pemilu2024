-- ============================================
-- MySQL Index Optimization for Pemilu 2024
-- Safe version with IF NOT EXISTS checks
-- ============================================

-- 1. Optimize pdpr_wil_tps (Polling Stations) - Most Critical
ALTER TABLE pdpr_wil_tps
  ADD INDEX IF NOT EXISTS idx_pro_kode (pro_kode),
  ADD INDEX IF NOT EXISTS idx_dapil_kode (dapil_kode),
  ADD INDEX IF NOT EXISTS idx_kab_kode (kab_kode),
  ADD INDEX IF NOT EXISTS idx_kec_kode (kec_kode),
  ADD INDEX IF NOT EXISTS idx_tps_kode (tps_kode),
  ADD INDEX IF NOT EXISTS idx_tps_id (tps_id),
  ADD INDEX IF NOT EXISTS idx_dapil_id (dapil_id);

-- Composite indexes for common JOIN patterns on TPS
ALTER TABLE pdpr_wil_tps
  ADD INDEX IF NOT EXISTS idx_pro_kab (pro_id, kab_id),
  ADD INDEX IF NOT EXISTS idx_kab_kec (kab_id, kec_id),
  ADD INDEX IF NOT EXISTS idx_kec_kel (kec_id, kel_id),
  ADD INDEX IF NOT EXISTS idx_pro_kab_kec (pro_id, kab_id, kec_id),
  ADD INDEX IF NOT EXISTS idx_kab_kec_kel (kab_id, kec_id, kel_id);

-- 2. Optimize pdpr_wil_pro (Provinces)
ALTER TABLE pdpr_wil_pro
  ADD INDEX IF NOT EXISTS idx_pro_id (pro_id),
  ADD INDEX IF NOT EXISTS idx_pro_kode (pro_kode);

-- 3. Optimize pdpr_wil_kab (Regencies)
ALTER TABLE pdpr_wil_kab
  ADD INDEX IF NOT EXISTS idx_kab_id (kab_id),
  ADD INDEX IF NOT EXISTS idx_kab_kode (kab_kode),
  ADD INDEX IF NOT EXISTS idx_pro_kab_composite (pro_id, kab_id);

-- 4. Optimize pdpr_wil_kec (Districts)
ALTER TABLE pdpr_wil_kec
  ADD INDEX IF NOT EXISTS idx_kec_id (kec_id),
  ADD INDEX IF NOT EXISTS idx_kec_kode (kec_kode),
  ADD INDEX IF NOT EXISTS idx_pro_id_kec (pro_id),
  ADD INDEX IF NOT EXISTS idx_dapil_id_kec (dapil_id),
  ADD INDEX IF NOT EXISTS idx_pro_kab_kec_composite (pro_id, kab_id, kec_id);

-- 5. Optimize pdpr_wil_kel (Villages)
ALTER TABLE pdpr_wil_kel
  ADD INDEX IF NOT EXISTS idx_kel_kode (kel_kode),
  ADD INDEX IF NOT EXISTS idx_pro_id_kel (pro_id),
  ADD INDEX IF NOT EXISTS idx_dapil_id_kel (dapil_id),
  ADD INDEX IF NOT EXISTS idx_pro_kab_kec_kel_composite (pro_id, kab_id, kec_id, kel_id);

-- 6. Optimize pdpr_wil_dapil (Electoral Districts)
ALTER TABLE pdpr_wil_dapil
  ADD INDEX IF NOT EXISTS idx_dapil_kode (dapil_kode),
  ADD INDEX IF NOT EXISTS idx_dapil_id_dapil (dapil_id),
  ADD INDEX IF NOT EXISTS idx_pro_dapil_composite (pro_id, dapil_id);

-- 7. Optimize DPRD Province tables
ALTER TABLE pdprdp_wil_tps
  ADD INDEX IF NOT EXISTS idx_pro_kode (pro_kode),
  ADD INDEX IF NOT EXISTS idx_kab_kode (kab_kode),
  ADD INDEX IF NOT EXISTS idx_kec_kode (kec_kode),
  ADD INDEX IF NOT EXISTS idx_kel_kode (kel_kode),
  ADD INDEX IF NOT EXISTS idx_pro_id (pro_id),
  ADD INDEX IF NOT EXISTS idx_kab_id (kab_id),
  ADD INDEX IF NOT EXISTS idx_kec_id (kec_id),
  ADD INDEX IF NOT EXISTS idx_kel_id (kel_id);

-- 8. Optimize DPRDK Aceh tables
ALTER TABLE pdprdk_wil_tps
  ADD INDEX IF NOT EXISTS idx_pro_kode (pro_kode),
  ADD INDEX IF NOT EXISTS idx_kab_kode (kab_kode),
  ADD INDEX IF NOT EXISTS idx_kec_kode (kec_kode),
  ADD INDEX IF NOT EXISTS idx_kel_kode (kel_kode),
  ADD INDEX IF NOT EXISTS idx_pro_id (pro_id),
  ADD INDEX IF NOT EXISTS idx_kab_id (kab_id),
  ADD INDEX IF NOT EXISTS idx_kec_id (kec_id),
  ADD INDEX IF NOT EXISTS idx_kel_id (kel_id);

-- 9. Optimize Candidate tables
ALTER TABLE dpr_ri_caleg
  ADD INDEX IF NOT EXISTS idx_dapil_kode (dapil_kode);

ALTER TABLE dprd_pro_caleg
  ADD INDEX IF NOT EXISTS idx_pro_id (pro_id),
  ADD INDEX IF NOT EXISTS idx_pro_kode (pro_kode),
  ADD INDEX IF NOT EXISTS idx_dapil_id (dapil_id),
  ADD INDEX IF NOT EXISTS idx_dapil_kode (dapil_kode),
  ADD INDEX IF NOT EXISTS idx_partai_id (partai_id),
  ADD INDEX IF NOT EXISTS idx_pro_dapil (pro_id, dapil_id);

ALTER TABLE dprd_kab_caleg
  ADD INDEX IF NOT EXISTS idx_pro_id (pro_id),
  ADD INDEX IF NOT EXISTS idx_kab_id (kab_id),
  ADD INDEX IF NOT EXISTS idx_kab_kode (kab_kode),
  ADD INDEX IF NOT EXISTS idx_dapil_id (dapil_id),
  ADD INDEX IF NOT EXISTS idx_dapil_kode (dapil_kode),
  ADD INDEX IF NOT EXISTS idx_partai_id (partai_id),
  ADD INDEX IF NOT EXISTS idx_kab_dapil (kab_id, dapil_id);

ALTER TABLE dpd_caleg
  ADD INDEX IF NOT EXISTS idx_pro_kode (pro_kode),
  ADD INDEX IF NOT EXISTS idx_nomor_urut (nomor_urut);

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
