# Backfill Script Template

Skeleton for a production-safe TypeScript backfill. Copy, fill in the `TODO` markers, and save to
`scripts/backfill/YYMMDD-<descriptive-name>.ts` before running.

**Run order:**
1. `DRY_RUN=true npx ts-node scripts/backfill/YYMMDD-<name>.ts` — inspect logs, no writes.
2. Review output. Fix logic if anything looks wrong.
3. `npx ts-node scripts/backfill/YYMMDD-<name>.ts` — writes to DB.

The script is idempotent. Re-running after a partial failure is safe.

---

```typescript
/**
 * Backfill: <one-line description of what this script populates>
 *
 * Safe to re-run — idempotent via `where: { <newField>: null }`.
 * Set DRY_RUN=true (default) to preview without writing.
 *
 * Usage:
 *   DRY_RUN=true npx ts-node scripts/backfill/YYMMDD-<name>.ts
 *   npx ts-node scripts/backfill/YYMMDD-<name>.ts
 */

// TODO: replace with your ORM client import
// Prisma:   import { PrismaClient } from '@prisma/client'
// TypeORM:  import { AppDataSource } from '../src/data-source'
// Drizzle:  import { db } from '../src/db'
// Mongoose: import '../src/database'; import { YourModel } from '../src/models/your.schema'
import { PrismaClient } from '@prisma/client'

// TODO: import existing domain validators / services / types so business logic is not duplicated
// import { computeNewField } from '../src/modules/your-domain/your-domain.service'

const DRY_RUN = process.env.DRY_RUN !== 'false'
const BATCH_SIZE = 500 // tune per table size and row weight

async function run(): Promise<void> {
  const client = new PrismaClient()
  // TODO: for TypeORM: await AppDataSource.initialize()
  // TODO: for Mongoose: await connectDatabase()

  console.log(`[backfill] DRY_RUN=${DRY_RUN}  BATCH_SIZE=${BATCH_SIZE}`)

  let processed = 0
  let cursor: string | undefined = undefined // use string for UUID PKs; number for int PKs

  while (true) {
    // TODO: replace `your_table` with the Prisma model / TypeORM repo / Drizzle table / Mongoose model
    // TODO: replace `new_field` with the column/field being backfilled
    // TODO: replace `id` with the PK field name
    const batch = await client.your_table.findMany({
      where: { new_field: null },      // idempotent: only rows not yet processed
      take: BATCH_SIZE,
      ...(cursor ? { skip: 1, cursor: { id: cursor } } : {}),
      orderBy: { id: 'asc' },
      select: {
        id: true,
        // TODO: select fields needed to compute new_field
      },
    })

    if (batch.length === 0) break

    cursor = batch[batch.length - 1].id

    for (const row of batch) {
      // TODO: compute the value for new_field
      // const newValue = computeNewField(row)
      const newValue = 'TODO_COMPUTE_VALUE'

      if (DRY_RUN) {
        console.log(`[dry-run] id=${row.id}  new_field=${newValue}`)
      } else {
        await client.your_table.update({
          where: { id: row.id },
          data: { new_field: newValue },
        })
      }
    }

    processed += batch.length
    console.log(`[backfill] processed ${processed} rows (last id: ${cursor})`)
  }

  console.log(`[backfill] done — total rows ${DRY_RUN ? 'scanned (dry-run)' : 'updated'}: ${processed}`)

  await client.$disconnect()
  // TODO: for TypeORM: await AppDataSource.destroy()
  // TODO: for Mongoose: await mongoose.disconnect()
}

run().catch((err) => {
  console.error('[backfill] fatal error:', err)
  process.exit(1)
})
```

---

## Adapting for other ORMs

### TypeORM

```typescript
import { AppDataSource } from '../src/data-source'
import { YourEntity } from '../src/entities/your.entity'

await AppDataSource.initialize()
const repo = AppDataSource.getRepository(YourEntity)

const batch = await repo.find({
  where: { newField: IsNull() },
  take: BATCH_SIZE,
  order: { id: 'ASC' },
})

if (!DRY_RUN) {
  await repo.update({ id: In(batch.map(r => r.id)) }, { newField: computedValue })
}
```

### Drizzle

```typescript
import { db } from '../src/db'
import { yourTable } from '../src/db/schema'
import { isNull, eq } from 'drizzle-orm'

const batch = await db
  .select()
  .from(yourTable)
  .where(isNull(yourTable.newField))
  .limit(BATCH_SIZE)

if (!DRY_RUN) {
  for (const row of batch) {
    await db.update(yourTable)
      .set({ newField: computeNewField(row) })
      .where(eq(yourTable.id, row.id))
  }
}
```

### Mongoose

```typescript
import { YourModel } from '../src/models/your.schema'

const batch = await YourModel
  .find({ newField: null })
  .limit(BATCH_SIZE)
  .lean()

if (!DRY_RUN) {
  const ops = batch.map(doc => ({
    updateOne: {
      filter: { _id: doc._id },
      update: { $set: { newField: computeNewField(doc) } },
    },
  }))
  await YourModel.bulkWrite(ops)
}
```

---

## Checklist before running on prod

- [ ] `DRY_RUN=true` run completed with no errors and expected log output
- [ ] Spot-checked a sample of the "would-update" rows — values look correct
- [ ] Migration deploying the new column/field has already been applied (`new_field` column exists)
- [ ] Existing domain logic/validators imported — no business rules re-implemented inline
- [ ] Batch size tuned — ran `EXPLAIN ANALYZE` (or equivalent) on the `where` query in staging
- [ ] Script committed to git so the change is auditable
- [ ] Run scheduled in a low-traffic window; progress logging visible
- [ ] Rollback plan documented (usually: `new_field = null` reset script, identical structure)
