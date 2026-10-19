-- Bind $1 as unrounded numeric; reject non-positive/fractional atomic amounts.
UPDATE account_balance
SET available = available - $1,
    version = version + 1
WHERE tenant_id = $2
  AND account_id = $3
  AND currency = $4
  AND $1 > 0
  AND $1 = trunc($1)
  AND available >= $1
RETURNING available, version;
