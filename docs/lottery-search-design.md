# Part 2 — Lottery Search System

Design proposal for searching a large lottery ticket dataset using wildcard pattern matching, with a distribution mechanism that prevents the same ticket from being returned to multiple users simultaneously.

No code implementation — design only, as specified.

---

## Table of Contents

- [Problem Analysis](#problem-analysis)
- [Assumptions](#assumptions)
- [Data Model](#data-model)
- [Search Algorithm](#search-algorithm)
- [Database Selection](#database-selection)
- [Distribution and Concurrency](#distribution-and-concurrency)
- [System Architecture](#system-architecture)
- [API Contract](#api-contract)
- [Performance Analysis](#performance-analysis)
- [Tradeoffs](#tradeoffs)
- [Failure Modes](#failure-modes)
- [Monitoring](#monitoring)
- [Scaling Path](#scaling-path)
- [Extension: Multiple Tickets Per Number](#extension-multiple-tickets-per-number)
- [Summary](#summary)

---

## Problem Analysis

Before choosing any technology, it is worth establishing how large this problem actually is.

### The dataset is smaller than it sounds

```
1,000,000 tickets, each a 6-digit number

as int32:   4 bytes × 1M  =  4 MB
as string:  6 bytes × 1M  =  6 MB
plus status flag          =  1 MB
                          ─────────
total                     ≈ 10 MB
```

Ten megabytes fits comfortably in the memory of the cheapest available instance. Any design that begins by reaching for a distributed search cluster has misjudged the scale.

### The pattern space

Each position in a search pattern can be one of eleven values — the digits `0` through `9` or the wildcard `*`:

```
11^6 = 1,771,561 possible patterns
```

This number is worth computing because it answers an obvious first idea: since the ticket set is fixed for the whole draw, why not precompute the answer for every possible pattern?

The count is feasible, but the storage is not. Summing results across all pattern widths requires storing roughly 60 million ticket references — around 240 MB, thirty times the cost of the index described later, to gain a latency improvement from 0.1 ms to 0.01 ms. Worse, precomputed results become stale the moment a ticket is reserved, so availability would have to be filtered at query time regardless.

Precomputation is therefore rejected, and the reasoning is recorded here rather than left unexamined.

### Result size depends only on wildcard count

```
number of wildcards = k  →  matches = 10^k
```

| Pattern | Wildcards | Matches |
| --- | --- | --- |
| `123456` | 0 | 1 |
| `12345*` | 1 | 10 |
| `1234**` | 2 | 100 |
| `123***` | 3 | 1,000 |
| `12****` | 4 | 10,000 |
| `1*****` | 5 | 100,000 |
| `******` | 6 | 1,000,000 |

The position of the wildcards is irrelevant to the result count — `1****5` and `****15` both match 10,000 numbers. This observation drives the choice of data structure, because a structure whose *performance* also ignores wildcard position is strictly better than one that does not.

### Three conclusions

**The hard problem is allocation, not search.** With the dataset in memory, matching is close to free. Every genuine difficulty in this exercise lives in requirement 3 — ensuring two users searching the same pattern at the same moment do not receive the same ticket.

**Broad patterns are the dangerous input.** `******` matches a million rows. Returning them is never acceptable, so a hard result cap is a correctness requirement rather than a nicety.

**Over-engineering is a failure mode here.** Proposing a five-node Elasticsearch cluster for 10 MB of fixed-format numeric data would demonstrate poor judgement about scale, which the evaluation criteria explicitly test under "real-world practicality."

---

## Assumptions

| # | Assumption | Basis |
| --- | --- | --- |
| 1 | One number maps to exactly one ticket | 1M tickets and 6-digit numbers align exactly with the range 000000–999999 |
| 2 | "Not returned to multiple users" means locking at display time, not at purchase time | The requirement says *returned*, which refers to search results; the specification also names *reservation* as an example mechanism |
| 3 | Reservations expire automatically after a short window (5 minutes) | Without expiry, a user who closes their browser would hold tickets indefinitely |
| 4 | The workload is heavily read-dominated | Users browse far more than they buy |
| 5 | The ticket set is fixed for the duration of a draw | Lotteries are issued per draw; no tickets are added mid-draw |

### A note on the stated data volume

The `main` branch of the challenge repository states 10 million tickets and `10M+` records, while the `doc/updateTest` branch states 1 million and `1M+`. This document follows the 1 million figure, under which the 6-digit space maps one-to-one onto the dataset and assumption 1 holds.

The design does not depend on that choice. The search index is built over *numbers*, of which there are always at most 1,000,000, so index size and search latency are unchanged at 10 million tickets or beyond. Only the allocation layer changes, and that case is covered in [Extension: Multiple Tickets Per Number](#extension-multiple-tickets-per-number).

Assumption 5 is the most consequential one. Because the ticket set does not change during a draw, the entire search index can be built once at startup and treated as immutable. Only availability status changes, and that is deliberately kept out of the index.

---

## Data Model

The design separates two concepts that are easy to conflate:

```
number  =  a 6-digit value; at most 1,000,000 of them exist   → drives the search index
ticket  =  a sellable unit                                    → drives inventory and allocation
```

Under assumption 1 these are one-to-one, but keeping them distinct is what allows the index to stay constant in size as the ticket count grows. Search operates on numbers; allocation operates on tickets.

A second property falls out of the numbering scheme: **the number is its own identifier.** `"012345"` converts directly to the integer `12345`, which serves as a bitmap position without any lookup table, hash, or surrogate key.

---

## Search Algorithm

Three structures were evaluated.

### Option 1 — Full scan with regex

Convert the pattern to a regular expression and test every ticket.

```
"1**4*6"  →  ^1..4.6$
```

Linear in the dataset: one million comparisons per query. On modern hardware with integer comparison this takes roughly 1–3 ms, which is not disqualifying on its own. It is the honest baseline every other option must beat.

It fails under concurrency. A thousand simultaneous searches means a billion comparisons per second, and CPU becomes the binding constraint long before anything else does.

### Option 2 — Trie

A six-level tree branching ten ways at each level. A digit follows one branch; a wildcard expands to ten.

```
"12****"  →  walk 1, walk 2, expand 10, 10, 10, 10
          →  ~11,110 nodes visited for 10,000 results     efficient

"****12"  →  expand 10, 100, 1,000, 10,000, then filter
          →  >100,000 nodes visited for 10,000 results    wasteful
```

The trie is excellent for trailing wildcards and poor for leading ones. The first example pattern in the specification is `****23` — precisely the trie's worst case. Adding a reversed suffix trie helps with that specific shape but still degrades on interior patterns such as `1**4*6`.

### Option 3 — Position-digit inverted index (recommended)

Rather than indexing whole numbers, index which digit occupies which position.

Build 60 bitmaps, one for each combination of position (0–5) and digit (0–9):

```
B[p][d] = the set of all numbers whose digit at position p equals d
```

Worked example over a small dataset:

```
position:   0 1 2 3 4 5
#1          1 2 3 4 5 5
#2          1 9 9 9 9 5
#3          2 2 2 2 2 2
#4          1 0 0 0 0 0
#5          9 9 9 9 9 5

B[0][1] = {#1, #2, #4}      position 0 holds digit 1
B[5][5] = {#1, #2, #5}      position 5 holds digit 5
```

A search is an intersection over the non-wildcard positions only:

```
pattern "1****5"
  position 0 is '1'  →  B[0][1]
  positions 1-4 are wildcards → skipped entirely
  position 5 is '5'  →  B[5][5]

  B[0][1] AND B[5][5] = {#1, #2}
```

Verified by inspection: `123455` and `199995` both start with 1 and end with 5; `100000` starts with 1 but ends with 0 and is correctly excluded.

Further examples:

```
"****23"  →  B[4][2] AND B[5][3]
"123***"  →  B[0][1] AND B[1][2] AND B[2][3]
"1**4*6"  →  B[0][1] AND B[3][4] AND B[5][6]
```

**Wildcards require no work at all — they are simply skipped.** This is the decisive property: unlike a trie, where a wildcard forces branching, here the position of a wildcard has no effect on cost whatsoever. Every query is at most six intersections. There is no worst case.

### Cost

```
one bitmap  =  1,000,000 bits  =  125 KB
60 bitmaps  =  7.5 MB
```

Intersection proceeds 64 bits at a time:

```
1,000,000 / 64          = 15,625 word operations per AND
at most 5 ANDs          = ~78,000 operations
```

Roughly 50–200 microseconds per query, consistently, for any pattern.

### Refinements

**Roaring bitmaps** compress sparse sets automatically and intersect in compressed form. Mature libraries exist for Go (`RoaringBitmap/roaring`) and most other languages.

**Order intersections by cardinality.** Starting with the smallest bitmap shrinks the intermediate result fastest and reduces total work — the same principle a query planner applies when ordering joins.

**Short-circuit on result count.** Since only the first N available tickets are returned, iteration over the result bitmap can stop as soon as N is reached rather than materialising all matches.

**Reject invalid patterns cheaply.** A valid pattern is exactly six characters, each of which is a digit or `*`. The expected result count `10^k` is known before any work is done, which allows the API to warn about or reject excessively broad patterns up front.

### Comparison

| | Full scan | Trie | **Inverted index** |
| --- | --- | --- | --- |
| Latency | ~1–3 ms | 0.1–50 ms | **~0.1 ms** |
| Sensitive to wildcard position | No | **Yes — severe** | No |
| Memory | 4 MB | ~50 MB | 7.5 MB |
| Build time | none | ~2 s | ~1 s |
| Implementation complexity | Very low | High | Moderate |
| Worst case | Constant | Pathological | **None** |

---

## Database Selection

### Evaluation criteria

1. Query latency for unanchored wildcard patterns
2. Support for atomic allocation under concurrency
3. Transactional correctness for the purchase path
4. Operational simplicity

### The core constraint, stated plainly

The difficulty with `****23` is not specific to any product. A B-tree index orders rows left to right, so a query that does not constrain the leftmost characters gives the index nothing to seek on. This is equally true of MongoDB, PostgreSQL, MySQL, and every other B-tree-backed store:

```
"123***"  →  ^123        anchored    → index seek        fast
"****23"  →  ^.{4}23$    unanchored  → full scan         slow
"1**4*6"  →  ^1..4.6$    unanchored  → full scan         slow
```

Two of the three example patterns in the specification are unanchored. A design that relies on B-tree indexing will fall back to scanning for most real queries.

### Option 1 — MongoDB

**Where it fits well.** A lottery platform carries substantial surrounding data — draws, vendors, agents, promotions, campaign metadata — whose shape changes over time, and a flexible schema handles that genuinely better than rigid DDL. The aggregation pipeline is well suited to sales reporting. Sharding is straightforward. A team already operating MongoDB will ship and troubleshoot faster on it than on anything new, and that is a real engineering consideration, not a soft one.

**Where it struggles for this particular workload.** Unanchored regex queries cannot use an index, producing a collection scan. The cost is higher than an equivalent in-process scan because each document must be decoded from BSON and the result set crosses the network.

**Reservation.** `findOneAndUpdate` with a status precondition is genuinely atomic and correct:

```javascript
db.tickets.findOneAndUpdate(
  { number: 123456, status: "available" },
  { $set: { status: "reserved", reservedBy: userId, expiresAt: ... } }
)
```

What is missing is expiry semantics suited to a hold. MongoDB's TTL index deletes whole documents and runs on a roughly 60-second cycle, which does not express "release this reservation in exactly five minutes." A background sweeper over `expiresAt` would have to be written and operated.

### Option 2 — PostgreSQL only

`LIKE '1____5'` works, but the same anchoring limitation applies and unanchored patterns produce sequential scans. Reservations via row locks hold transactions open across user think-time, consuming connections and creating contention under load.

### Option 3 — Elasticsearch

Built for full-text retrieval over large, loosely structured corpora. This dataset is 10 MB of fixed-width numerics. Cluster operation costs would dominate, and query latency would still exceed an in-process bitmap intersection because of network and JSON overhead.

### Option 4 — Redis only

Redis can store bitmaps and intersect them with `BITOP AND`. Two problems: results must cross the network on every query, and Redis is single-threaded, so CPU-bound intersections would block the reservation commands that must stay fast. Redis should not be given compute-heavy work in this design.

### Recommendation — a layered approach

No single store is well suited to all three workloads, and the workloads have genuinely different shapes:

| Data | Character | Requirement |
| --- | --- | --- |
| Ticket numbers | Fixed for the draw | Fast read, never written |
| Search index | Built once at startup | Resident in memory |
| Reservation state | Changes constantly | Atomic operations with TTL |
| Orders and payments | Money | ACID |

```
┌──────────────────────────────────────────────────┐
│  In-process memory (inside each API instance)    │
│  60 roaring bitmaps — 7.5 MB                     │
│  Built from the database at startup              │
│  Responsibility: pattern matching                │
└──────────────────────────────────────────────────┘
                        │
┌──────────────────────────────────────────────────┐
│  Redis                                           │
│  lock:<number> → userId, TTL 300s                │
│  Responsibility: reservation, atomic allocation  │
└──────────────────────────────────────────────────┘
                        │
┌──────────────────────────────────────────────────┐
│  Primary database (MongoDB or PostgreSQL)        │
│  tickets, orders, payments                       │
│  Responsibility: source of truth, transactions   │
└──────────────────────────────────────────────────┘
```

**Why in-memory for the index.** 7.5 MB of immutable data. Keeping it in the service's own heap eliminates network hops and serialisation entirely. Each instance holds its own copy, but since the data never changes during a draw there is no cache coherence problem — 7.5 MB across ten instances is 75 MB in total, which is negligible.

**Why Redis for reservations.** One command provides everything required:

```
SET lock:123456 <userId> NX EX 300
```

`NX` makes it an atomic compare-and-set. `EX 300` provides automatic release. No transaction, no polling, no cleanup job, no distributed lock algorithm.

**Why a relational or document store remains the source of truth.** The purchase path handles money and requires real transactional guarantees. A uniqueness constraint or conditional update at this layer is the final barrier against double-selling, and it holds even if Redis is entirely unavailable.

**This recommendation adds a layer rather than replacing one.** If MongoDB is already in production, it stays: the bitmaps are built from it at startup, Redis holds only ephemeral locks, and MongoDB remains the owner of all durable truth. Nothing is migrated away.

### If constrained to a single database

PostgreSQL, chosen because transactional correctness matters more than search latency, with these mitigations:

- Store numbers as `INTEGER` rather than text
- Add six generated columns, one per digit position, and index each — approximating the inverted index through PostgreSQL's own bitmap index scans

```sql
digit0 SMALLINT GENERATED ALWAYS AS (number / 100000 % 10) STORED
digit1 SMALLINT GENERATED ALWAYS AS (number /  10000 % 10) STORED
...
```

- Use advisory locks (`pg_try_advisory_lock(number)`) rather than row locks for reservations, avoiding long-held transactions

This is roughly 10–30× slower than the layered design, landing in the 5–10 ms range — still perfectly usable — while reducing the number of systems to operate from three to one. For many organisations that is the right trade.

---

## Distribution and Concurrency

This section addresses requirement 3, which is the substance of the exercise.

### The problem

```
time   User A                      User B
──────────────────────────────────────────────────────
t=0    search "****23"
t=0.1  receives 123423, 456723...
t=0.2                              search "****23"
t=0.3                              receives 123423, 456723...   identical
t=5    buys 123423 — succeeds
t=6                                buys 123423 — rejected
```

User B spent six seconds choosing a ticket that was never available. With many concurrent users this becomes the common case rather than the exception.

### Approach — reserve on read

Treat a search as an allocation rather than a read.

```
1. Intersect bitmaps            → candidate numbers
2. Attempt to lock candidates   → atomically, in one batch
3. Return only what was locked  → with an expiry timestamp
```

User A receives twenty tickets already held for them. User B, searching the same pattern moments later, receives the next twenty. No overlap is possible.

### The primitive

```
SET lock:123456 <userId> NX EX 300
```

| Component | Purpose |
| --- | --- |
| `lock:123456` | The number is the key — a direct consequence of one number per ticket |
| `<userId>` | Records the holder, checked before purchase and before release |
| `NX` | Sets only if absent; returns nil if already held |
| `EX 300` | Automatic release after five minutes |

Redis executes commands one at a time. If a thousand requests issue `SET lock:123456 NX` simultaneously, exactly one receives `OK` and the rest receive nil. Correctness under concurrency comes from the data store's own execution model rather than from application coordination.

### Batching

Twenty individual round trips would cost roughly 20 ms. A single pipeline costs about 1 ms:

```
PIPELINE
  SET lock:123423 userA NX EX 300
  SET lock:456723 userA NX EX 300
  ... (send ~40 candidates for a target of 20)
EXEC

→ keep the successes, truncate to 20
```

Candidates are deliberately oversupplied, because some will already be held by other users. The oversupply factor should adapt to the observed success rate as the draw sells through.

### Ticket lifecycle

```
        ┌───────────┐
        │ AVAILABLE │◄──────────────┐
        └─────┬─────┘               │
              │ matched + SET NX    │ TTL expiry
              ▼                     │ or explicit release
        ┌───────────┐               │
        │ RESERVED  │───────────────┘
        └─────┬─────┘
              │ payment completes
              ▼
        ┌───────────┐
        │   SOLD    │   terminal
        └───────────┘
```

`RESERVED` exists only in Redis. `SOLD` is written to the database. This split matters: persisting reservations would mean a database write for every search, which would defeat the entire design.

### Purchase path

```
1. Verify lock:<number> still belongs to this user
   → otherwise 409, reservation expired

2. BEGIN
     UPDATE tickets SET status='SOLD', owner_id=..., sold_at=now()
     WHERE number = <n> AND status = 'AVAILABLE'
     
     → if zero rows affected: ROLLBACK, return 409
     
     INSERT INTO orders ...
   COMMIT

3. Release the Redis lock via a Lua script that checks ownership
4. Publish the sale so other instances update their sold bitmap
```

**Step 2 is the guarantee.** The `AND status = 'AVAILABLE'` predicate means that even if Redis loses every lock, double-selling remains impossible — the database refuses the second update. Every layer above may fail; this one may not.

**Step 3 requires Lua rather than `DEL`:**

```lua
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
```

A plain `DEL` risks this sequence: A holds a lock, the TTL expires, B acquires it, A's delayed release then removes B's lock. Redis executes Lua scripts atomically, so ownership can be verified and the key deleted as one indivisible step.

### Filtering sold tickets from results

**Option A — maintain a sold bitmap in memory.**

```
result = B[4][2] AND B[5][3] AND NOT sold
```

Fastest, but requires propagation across instances via pub/sub and tolerates brief inconsistency.

**Option B — let Redis do the filtering.**

No sold bitmap at all. `SET NX` fails for held tickets, and the database rejects sold ones at purchase. Simpler, at the cost of larger candidate batches once a draw is mostly sold.

**Recommendation: start with B, measure the reservation hit rate, and add A only if it degrades.** The simpler design is correct from the outset; the optimisation can be introduced against evidence.

### Abuse prevention

The reservation mechanism can itself be turned into a denial of service. A single user issuing a hundred searches would hold thousands of tickets for five minutes, making them invisible to everyone else.

| Control | Value | Purpose |
| --- | --- | --- |
| Results per query | ≤ 50 | Caps locks per request, including for `******` |
| Concurrent holds per user | ≤ 50 | Tracked in `user:<id>:holds`, checked before locking |
| Search rate limit | ~10/min/user | Prevents sweeping |
| Reservation TTL | 300s, reduced under load | Balances decision time against inventory turnover |

Explicit release on user action (choosing a different ticket, abandoning a cart) returns inventory immediately rather than waiting for expiry.

### Alternatives considered

| | Mechanism | Advantage | Disadvantage | Best when |
| --- | --- | --- | --- | --- |
| **A — Reserve on read** (chosen) | `SET NX EX` during search | No collisions; best experience | Most complex; tickets held by non-buyers | Contention is high, e.g. desirable numbers |
| B — Lock on purchase | Conditional update at checkout | Very simple; no held state | Users choose, then get rejected | Supply is plentiful relative to demand |
| C — Partitioned results | Deterministic split of candidates by session hash | Stateless; no locks at all | Users see an incomplete view; some tickets may never surface | Traffic is extreme and completeness is negotiable |

**Why A.** The requirement says the same ticket should not be *returned* to multiple users, which refers to search results rather than to the purchase step, and the specification names *reservation* among its suggested mechanisms.

**A hybrid is worth considering in production.** Narrow patterns (fewer than ~100 matches) indicate high contention and warrant reserve-on-read; broad patterns have a low collision probability and can safely fall back to lock-on-purchase, avoiding large speculative holds.

---

## System Architecture

```
                      ┌──────────────┐
                      │    Client    │
                      └──────┬───────┘
                      ┌──────▼───────┐
                      │ Load balancer│
                      └──────┬───────┘
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │  API #1  │   │  API #2  │   │  API #3  │   stateless
        │ bitmaps  │   │ bitmaps  │   │ bitmaps  │   7.5 MB each
        └────┬─────┘   └────┬─────┘   └────┬─────┘
             └──────────────┼──────────────┘
                    ┌───────┴────────┐
                    ▼                ▼
              ┌──────────┐    ┌─────────────┐
              │  Redis   │    │  Database   │
              │  locks   │    │  tickets    │
              │  TTL 5m  │    │  orders     │
              └──────────┘    └─────────────┘
```

Instances are stateless despite holding bitmaps in memory, because those bitmaps are immutable derived data rather than session state. Scaling out requires no coordination: a new instance loads its own copy at startup.

### Startup sequence

```
1. Read all tickets for the current draw from the database    ~1 s
2. Construct 60 roaring bitmaps                               ~1 s
3. Build the sold bitmap from already-sold tickets
4. Warm caches with a few representative queries
5. Pass the readiness probe and begin accepting traffic
```

The readiness probe must not pass before step 4, or the load balancer will route traffic to an instance that cannot yet answer.

### Cross-instance propagation

When a sale completes, the selling instance publishes the number and the others update their sold bitmaps. Propagation lag of a few milliseconds is acceptable: a stale bitmap can only cause an unavailable ticket to be offered, and it will be rejected at the Redis or database layer. Correctness is never at risk — only presentation quality.

---

## API Contract

### `POST /api/v1/tickets/search`

Searches and reserves in a single call.

```json
{ "pattern": "****23", "limit": 20 }
```

```json
{
  "pattern": "****23",
  "total_matches": 10000,
  "available_estimate": 8734,
  "returned": 20,
  "reservation_expires_at": "2026-09-12T15:05:00Z",
  "tickets": [
    { "number": "123423", "reservation_id": "r_a1b2c3" },
    { "number": "456723", "reservation_id": "r_d4e5f6" }
  ]
}
```

`total_matches` is computed as `10^k` from the wildcard count without touching the index, and can be used to warn about overly broad patterns before any work is performed.

`available_estimate` is named as an estimate deliberately. Availability changes continuously, and producing an exact figure would require locking. Clients must not treat it as authoritative; confirmation happens only at purchase.

`reservation_expires_at` allows the client to display a countdown.

### `POST /api/v1/tickets/purchase`

```json
{ "reservation_ids": ["r_a1b2c3"] }
```

| Status | Condition |
| --- | --- |
| 201 | Purchase completed |
| 409 | Reservation expired, or the ticket was already sold |
| 403 | The reservation belongs to another user |

### `DELETE /api/v1/reservations/{id}`

Releases a hold immediately rather than waiting for expiry, returning inventory to circulation as soon as the user moves on.

### `GET /api/v1/tickets/count?pattern=****23`

Counts without reserving. This endpoint exists specifically so that exploratory interactions — autocomplete, live result counts while typing — do not lock inventory.

---

## Performance Analysis

### Latency for a single search

| Stage | Time | Note |
| --- | --- | --- |
| Parse and validate pattern | < 1 µs | Six character checks |
| Bitmap intersection (≤ 6) | 50–200 µs | Independent of wildcard position |
| Extract N results | ~10 µs | Stops at N |
| Redis pipeline (~40 locks) | 0.5–1 ms | One round trip |
| JSON serialisation | ~50 µs | |
| **Total, p50** | **~1 ms** | |
| **Total, p99** | **~5 ms** | |

**Redis dominates the budget, not the search.** A single network round trip costs roughly ten times a full bitmap intersection. This is worth stating explicitly because it identifies where further optimisation should be directed: reducing round trips, not refining the matching algorithm.

### Throughput

```
per core          ~1,000 searches/s   (bounded by Redis round trip)
per instance      ~4,000 searches/s   (4 cores)
three instances  ~12,000 searches/s
```

A single Redis node handles roughly 100,000 operations per second. At approximately 40 operations per search, that supports around 2,500 searches per second — meaning **Redis becomes the binding constraint before the API tier does.** Beyond that point, sharding Redis by number range distributes the load, and the lock keyspace partitions cleanly because every key is a number.

### Comparison against alternatives

| Approach | p50 | Memory | Note |
| --- | --- | --- | --- |
| **Bitmap intersection** | **0.1 ms** | 7.5 MB | Constant across patterns |
| In-memory full scan | 1–3 ms | 4 MB | Linear in dataset size |
| Trie | 0.1–50 ms | ~50 MB | Degrades on leading wildcards |
| PostgreSQL `LIKE` | 50–200 ms | — | Sequential scan when unanchored |
| MongoDB regex | 200–800 ms | — | Collection scan plus BSON decode |
| Elasticsearch | 10–50 ms | ~500 MB | Disproportionate for this scale |

These figures are estimates derived from the characteristics of each approach rather than measured benchmarks, and are presented as such.

### The scaling property that matters

Because the index is built over numbers rather than tickets, its size is bounded by the 6-digit space regardless of how many tickets exist:

```
1 million tickets    →  index 7.5 MB
10 million tickets   →  index 7.5 MB
100 million tickets  →  index 7.5 MB
```

Scan-based approaches have no equivalent property — their cost rises linearly with ticket count. This directly addresses the `1M+` qualifier in the requirement: the design does not merely meet the stated volume, it is insensitive to growth in it.

---

## Tradeoffs

| Decision | Gained | Given up |
| --- | --- | --- |
| Bitmaps in each instance's memory | Lowest achievable latency; no network | Per-instance copy; 2–3 s startup |
| Reserve on read | Users never lose a ticket after choosing it | Non-buyers hold inventory; availability temporarily reduced |
| 5-minute TTL | Reasonable decision window | Too short frustrates users; too long slows turnover |
| Redis separate from the database | Atomicity and expiry for free | An additional system to operate; reconciliation required |
| Three-layer architecture | Each layer does what it is good at | More complexity than a single store; three systems to monitor |
| Estimated availability count | No locking needed for counting | Clients must tolerate approximation |

**The trade deserving most attention is inventory held by non-buyers.** With a thousand concurrent users holding twenty tickets each, twenty thousand tickets — two percent of the draw — are unavailable at any moment. That is acceptable. At ten thousand concurrent users it becomes twenty percent, which is not.

The mitigation is an adaptive TTL: as the proportion of locked inventory rises, shorten the reservation window automatically. This keeps the mechanism's cost proportional to actual demand rather than fixed at its worst case.

---

## Failure Modes

| Failure | Impact | Behaviour |
| --- | --- | --- |
| Redis unavailable | Reservation impossible | Search continues normally from in-memory bitmaps. Degrade to browse-only, or allow purchases and let the database arbitrate — some users will receive 409 |
| Database unavailable | Purchase impossible | Search continues; purchases must be disabled. Never fall back to trusting Redis for a money path |
| API instance dies | None | Stateless; the load balancer removes it and its held locks expire naturally |
| Keyspace notification missed | Counters drift | Reconciliation repairs within one cycle |
| Network partition to Redis | Affected instances cannot reserve | Short timeout (~100 ms) and fail fast rather than queueing requests |
| Sold bitmap stale | An unavailable ticket is displayed | No correctness impact — rejected at Redis or at the database |

**The principle running through all of these:** every layer above the database is permitted to be wrong. The database is not. The `WHERE status = 'AVAILABLE'` predicate is the single guarantee against double-selling, and it holds regardless of what fails above it.

This mirrors the reasoning applied in Part 1, where an application-level uniqueness check exists for a clean error message while the unique index is what actually enforces the constraint. Checks above exist for user experience; the constraint at the bottom exists for correctness.

---

## Monitoring

| Metric | Why it matters |
| --- | --- |
| Search latency p50 / p99 | Detects regression |
| Reservation hit rate | A decline means candidate batches are too small |
| Lock utilisation (% of inventory held) | High values call for a shorter TTL |
| Reservation-to-purchase conversion | Low values indicate wasted holds |
| Reconciliation drift | Growth indicates a problem with expiry handling |
| Redis operations per second | The next bottleneck to appear |

---

## Scaling Path

```
Current (1M tickets, ~10k searches/s)
  3 API instances, single Redis, single database

10× volume
  Index unchanged at 7.5 MB
  Add API instances
  Shard Redis by number range
  Partition ticket storage by draw

100× volume
  Split search and purchase into separate services
  Search is read-only and scales without limit
  Purchase requires consistency and scales more conservatively
```

---

## Extension: Multiple Tickets Per Number

Assumption 1 holds for this dataset but does not describe every real lottery, and the `main` branch of the challenge states a volume under which it cannot hold. The design accommodates the change with one layer affected.

### Unchanged

The entire search layer. Bitmaps index numbers, and there are at most 1,000,000 numbers regardless of how many physical tickets exist. Index size, query latency, and the matching algorithm are all unaffected.

### Changed

The question shifts from a boolean to a count:

```
before:  is number 123456 taken?          yes / no
after:   how many of number 123456 remain?  counter
```

`SET NX` is replaced by an atomic counter:

```
DECR avail:123456
  ≥ 0  →  reservation granted
  < 0  →  sold out; INCR to restore immediately
```

`DECR` is atomic in the same way `SET NX` is — concurrent callers receive distinct decreasing values, so no two users can be granted the same unit.

**A new problem appears: counters have no expiry semantics.** A `SET NX EX` key releases itself; a decremented counter does not increment itself when a reservation lapses. This is handled in two layers:

- **Keyspace notifications.** Subscribe to expiry events on reservation keys and restore the counter when one fires.
- **Reconciliation.** Run periodically, because notifications can be lost if a subscriber restarts:

```
avail:N  should equal  total(N) − sold(N) − active_holds(N)
                       └ database ┘        └── Redis ──┘
```

This is possible precisely because the database remains the source of truth; Redis is a fast cache that can always be rebuilt from it.

**Allocating a specific physical ticket** moves to purchase time:

```sql
SELECT id FROM tickets
WHERE number = 123456 AND status = 'AVAILABLE'
ORDER BY id LIMIT 1
FOR UPDATE SKIP LOCKED;
```

`SKIP LOCKED` steps over rows another transaction is holding rather than waiting behind them, so a hundred concurrent buyers of the same number each receive a different ticket without queueing.

**Requirement 3 becomes substantially easier in this variant.** Two users seeing the same number simultaneously is no longer a conflict — there is stock for both. Contention only arises as a number approaches sell-out, which makes lock-on-purchase (alternative B) a defensible choice in its own right.

---

## Summary

| Requirement | Approach |
| --- | --- |
| 1 — Data volume | 1M numbers indexed as 60 bitmaps totalling 7.5 MB, held in process |
| 2 — Wildcard patterns | Position-digit inverted index; wildcards are skipped, so cost is independent of their position |
| 3 — Result distribution | Reserve-on-read via `SET NX EX`, with per-user caps and automatic expiry |
| 4 — Performance | ~0.1 ms matching, ~1 ms end to end; index size constant as ticket count grows |
| 5 — Production design | Layered — memory for search, Redis for allocation, existing database as source of truth |

Three decisions carry most of the design:

**Indexing digit positions rather than whole numbers** removes the asymmetry every tree-based structure suffers, where leading wildcards cost far more than trailing ones. Every pattern costs the same.

**Indexing numbers rather than tickets** decouples index size from dataset size. Ten times the tickets does not mean ten times the index.

**Placing the final correctness guarantee in the database** allows every faster layer above it to be an optimisation rather than a dependency. Redis can fail, bitmaps can go stale, and tickets still cannot be sold twice.
