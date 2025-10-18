-- ============================================
-- MySQL Index Optimization for Pemilu 2024
-- Final version - manually skip duplicates
-- ============================================

-- 1. Optimize pdpr_wil_tps (Polling Stations) - Most Critical Table
-- Adding missing code indexes
ALTER TABLE pdpr_wil_tps ADD INDEX idx_pro_kode (pro_kode);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_dapil_kode (dapil_kode);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_kab_kode (kab_kode);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_kec_kode (kec_kode);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_tps_kode (tps_kode);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_tps_id (tps_id);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_dapil_id (dapil_id);

-- Adding composite indexes for faster JOINs
ALTER TABLE pdpr_wil_tps ADD INDEX idx_pro_kab (pro_id, kab_id);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_kab_kec (kab_id, kec_id);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_kec_kel (kec_id, kel_id);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_pro_kab_kec (pro_id, kab_id, kec_id);
ALTER TABLE pdpr_wil_tps ADD INDEX idx_kab_kec_kel (kab_id, kec_id, kel_id);

-- 2. Optimize pdpr_wil_pro (Provinces)
ALTER TABLE pdpr_wil_pro ADD INDEX idx_pro_id (pro_id);
ALTER TABLE pdpr_wil_pro ADD INDEX idx_pro_kode (pro_kode);

-- 3. Optimize pdpr_wil_kab (Regencies)
ALTER TABLE pdpr_wil_kab ADD INDEX idx_kab_id (kab_id);
ALTER TABLE pdpr_wil_kab ADD INDEX idx_kab_kode (kab_kode);
ALTER TABLE pdpr_wil_kab ADD INDEX idx_pro_kab_composite (pro_id, kab_id);

-- 4. Optimize pdpr_wil_kec (Districts)
ALTER TABLE pdpr_wil_kec ADD INDEX idx_kec_id (kec_id);
ALTER TABLE pdpr_wil_kec ADD INDEX idx_kec_kode (kec_kode);
ALTER TABLE pdpr_wil_kec ADD INDEX idx_pro_id_kec (pro_id);
ALTER TABLE pdpr_wil_kec ADD INDEX idx_dapil_id_kec (dapil_id);
ALTER TABLE pdpr_wil_kec ADD INDEX idx_pro_kab_kec_composite (pro_id, kab_id, kec_id);

-- 5. Optimize pdpr_wil_kel (Villages)
ALTER TABLE pdpr_wil_kel ADD INDEX idx_kel_kode (kel_kode);
ALTER TABLE pdpr_wil_kel ADD INDEX idx_pro_id_kel (pro_id);
ALTER TABLE pdpr_wil_kel ADD INDEX idx_dapil_id_kel (dapil_id);
ALTER TABLE pdpr_wil_kel ADD INDEX idx_pro_kab_kec_kel_composite (pro_id, kab_id, kec_id, kel_id);

-- 6. Optimize pdpr_wil_dapil (Electoral Districts)
ALTER TABLE pdpr_wil_dapil ADD INDEX idx_dapil_kode (dapil_kode);
ALTER TABLE pdpr_wil_dapil ADD INDEX idx_dapil_id_dapil (dapil_id);
ALTER TABLE pdpr_wil_dapil ADD INDEX idx_pro_dapil_composite (pro_id, dapil_id);

-- 7. Optimize DPRD Province tables
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_pro_kode_pdprdp (pro_kode);
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_kab_kode_pdprdp (kab_kode);
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_kec_kode_pdprdp (kec_kode);
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_kel_kode_pdprdp (kel_kode);
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_pro_id_pdprdp (pro_id);
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_kab_id_pdprdp (kab_id);
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_kec_id_pdprdp (kec_id);
ALTER TABLE pdprdp_wil_tps ADD INDEX idx_kel_id_pdprdp (kel_id);

-- 8. Optimize DPRDK Aceh tables
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_pro_kode_pdprdk (pro_kode);
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_kab_kode_pdprdk (kab_kode);
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_kec_kode_pdprdk (kec_kode);
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_kel_kode_pdprdk (kel_kode);
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_pro_id_pdprdk (pro_id);
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_kab_id_pdprdk (kab_id);
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_kec_id_pdprdk (kec_id);
ALTER TABLE pdprdk_wil_tps ADD INDEX idx_kel_id_pdprdk (kel_id);

-- 9. Optimize Candidate tables
ALTER TABLE dpr_ri_caleg ADD INDEX idx_dapil_kode_caleg (dapil_kode);

ALTER TABLE dprd_pro_caleg ADD INDEX idx_pro_id_caleg (pro_id);
ALTER TABLE dprd_pro_caleg ADD INDEX idx_pro_kode_caleg (pro_kode);
ALTER TABLE dprd_pro_caleg ADD INDEX idx_dapil_id_caleg (dapil_id);
ALTER TABLE dprd_pro_caleg ADD INDEX idx_dapil_kode_caleg (dapil_kode);
ALTER TABLE dprd_pro_caleg ADD INDEX idx_partai_id_caleg (partai_id);
ALTER TABLE dprd_pro_caleg ADD INDEX idx_pro_dapil_caleg (pro_id, dapil_id);

ALTER TABLE dprd_kab_caleg ADD INDEX idx_pro_id_kab_caleg (pro_id);
ALTER TABLE dprd_kab_caleg ADD INDEX idx_kab_id_kab_caleg (kab_id);
ALTER TABLE dprd_kab_caleg ADD INDEX idx_kab_kode_kab_caleg (kab_kode);
ALTER TABLE dprd_kab_caleg ADD INDEX idx_dapil_id_kab_caleg (dapil_id);
ALTER TABLE dprd_kab_caleg ADD INDEX idx_dapil_kode_kab_caleg (dapil_kode);
ALTER TABLE dprd_kab_caleg ADD INDEX idx_partai_id_kab_caleg (partai_id);
ALTER TABLE dprd_kab_caleg ADD INDEX idx_kab_dapil_caleg (kab_id, dapil_id);

ALTER TABLE dpd_caleg ADD INDEX idx_pro_kode_dpd (pro_kode);
ALTER TABLE dpd_caleg ADD INDEX idx_nomor_urut_dpd (nomor_urut);

-- ============================================
-- Analyze tables to update statistics
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
