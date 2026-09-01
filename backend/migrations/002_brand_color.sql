-- Karecik — brand colour update
--
-- The main theme colour changed from green (#1a7f5a) to the corporate blue
-- (#1d4ed8). The column default in 001_init.sql was updated for fresh installs;
-- this migration brings ALREADY CREATED databases in line.
--
-- Only rows still carrying the old default are touched. A business that picked
-- its own colour in the dashboard (e.g. #7c3aed) is left alone.

ALTER TABLE businesses ALTER COLUMN primary_color SET DEFAULT '#1d4ed8';

UPDATE businesses
SET primary_color = '#1d4ed8'
WHERE primary_color = '#1a7f5a';
