-- Karecik — marka rengi güncellemesi
--
-- Ana tema rengi yeşilden (#1a7f5a) kurumsal maviye (#1d4ed8) çevrildi.
-- 001_init.sql'deki sütun varsayılanı yeni kurulumlar için güncellendi;
-- bu migration ise ZATEN OLUŞTURULMUŞ veritabanlarını hizaya getirir.
--
-- Yalnızca eski varsayılanı taşıyan kayıtlar değişir. İşletme panelden
-- kendi rengini seçtiyse (ör. #7c3aed) ona dokunulmaz.

ALTER TABLE businesses ALTER COLUMN primary_color SET DEFAULT '#1d4ed8';

UPDATE businesses
SET primary_color = '#1d4ed8'
WHERE primary_color = '#1a7f5a';
