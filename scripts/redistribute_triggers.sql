-- redistribute_triggers.sql
--
-- One-shot migration: spread existing users' trigger_minute across the
-- 22:00–22:10 window so they don't all fire at 22:02. Compressed to 10
-- minutes (not 28) so that if auto-sign silently fails, users still have
-- until 22:30 to sprint home from a teaching building and check in by hand.
--
-- Only updates users still on the legacy default (trigger_minute=2,
-- jitter_sec=180). Users who manually changed their schedule keep what
-- they had.
--
-- ⚠️ Run with wangui container stopped (docker compose stop) to avoid
-- racing with the scheduler's own writes.
--
-- Usage on the server:
--   cd /root/wangui
--   docker compose stop
--   sqlite3 data/wangui.db < scripts/redistribute_triggers.sql
--   docker compose start

.headers on
.mode column

-- --- snapshot before ---
SELECT 'BEFORE' AS phase, user_id, user_name,
       trigger_minute, jitter_sec
FROM users
ORDER BY trigger_minute, user_name;

-- --- redistribute ---
-- abs(random()) % 10 → uniform [0, 9]. Matches the new activate handler
-- (rand.IntN(10)). With JitterSec=60 the actual sign time falls within
-- 22:00:00 – 22:10:59, leaving 19+ minutes of human bail-out window.
--
-- Rolls users in two cohorts:
--   1. trigger_minute = 2  → legacy untouched default
--   2. trigger_minute > 9  → previously redistributed across 0..27, now
--                            need to be squeezed back into 0..9
-- Users currently sitting at 0..9 (manually set or already correct) are
-- left alone.
UPDATE users
SET    trigger_minute = abs(random()) % 10
WHERE  trigger_minute > 9 OR trigger_minute = 2;

-- Bring jitter in line with the new 60-second default. Users who manually
-- bumped it (or set it to 0) are not touched.
UPDATE users
SET    jitter_sec = 60
WHERE  jitter_sec = 180;

-- --- snapshot after ---
SELECT 'AFTER ' AS phase, user_id, user_name,
       trigger_minute, jitter_sec
FROM users
ORDER BY trigger_minute, user_name;
