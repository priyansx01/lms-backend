# SmartFM LMS — System Architecture Document

**Version:** 1.0  
**Date:** April 2026  
**Stack:** Go (Golang) · PostgreSQL · ClickHouse · Redis · Kafka · MinIO · AWS  
**Prepared for:** Internal distribution — SmartFM Engineering

---

## 1. Overview

SmartFM LMS is an in-house Learning Management System built for a housekeeping company, designed to train employees through video-based courses, quizzes, and performance tracking. It integrates with the existing SmartFM HRMS for identity but operates as a fully independent service with its own user IDs — allowing the web portal to be accessed by anyone with an LMS credential, not just HRMS-registered employees.

### Design Principles

- **Decoupled identity** — SmartFM HRMS is the source of truth for employees; LMS has its own `lms_user_id` namespace
- **Budget-first scalability** — services scale to zero when idle, object storage over managed databases where possible
- **Async-first video pipeline** — uploads never block the API; all transcoding is event-driven via Kafka
- **Go everywhere** — single binary deployments, low memory footprint, fast startup

---

## 2. High-Level Architecture

### 2.1 System Layers

| Layer | Components |
|---|---|
| External systems | SmartFM HRMS, Web Portal (LMS ID login) |
| Entry layer | API Gateway (Go) — Auth, JWT, rate limiting, routing |
| Core services | User Service, LMS Service, Assessment Service, Analytics Service |
| Data stores | Redis (sessions), PostgreSQL (courses, scores), ClickHouse (analytics) |
| Video pipeline | MinIO → Kafka → FFmpeg workers → MinIO HLS → AWS CloudFront |
| Support services | Notification Service, Leaderboard Service, Video Analytics Engine |

### 2.2 Core Microservices

**User Service** — Manages employee profiles and the `smartfm_id → lms_user_id` mapping. On first login via HRMS SSO, the gateway creates an LMS identity and issues a JWT. All downstream services use only `lms_user_id`.

**LMS Service** — Handles course creation, module management, and content metadata. Stores structured data in PostgreSQL. Issues presigned MinIO URLs for video uploads.

**Assessment Service** — Manages quizzes, scoring, pass/fail logic, and attempt tracking. Decoupled from LMS so quiz logic can evolve independently.

**Analytics Service** — Receives events (video watched, quiz attempted, drop-off timestamps) and writes them to ClickHouse for fast aggregation queries.

---

## 3. Identity & Authentication

### 3.1 HRMS Login Flow

```
SmartFM HRMS login
  → API Gateway issues JWT (smartfm_id → lms_user_id mapping)
  → User Service upserts profile in PostgreSQL
  → JWT cached in Redis (TTL: 24h)
```

### 3.2 Web Portal Login (LMS ID direct)

Any user with an `lms_user_id` and password can log in directly through the web portal — no HRMS dependency. This supports external trainees, contractors, or future expansion beyond HRMS-registered employees.

### 3.3 JWT Payload

```json
{
  "lms_user_id": "uuid",
  "smartfm_id": "optional — present only for HRMS-linked accounts",
  "role": "employee | instructor | admin",
  "exp": 1234567890
}
```

---

## 4. Course Creation & Video Pipeline

### 4.1 Upload Flow

1. Instructor calls `POST /courses` with title, description, and metadata
2. LMS Service creates a course row in PostgreSQL (status: `pending`)
3. LMS Service returns a **presigned MinIO URL** — instructor uploads raw video directly to object storage, bypassing the API server
4. MinIO triggers a Kafka event on the `video.uploaded` topic
5. FFmpeg Worker picks up the event and begins transcoding

### 4.2 Kafka Event Payload

```json
{
  "course_id": "uuid",
  "lms_user_id": "uuid",
  "minio_key": "raw/course_id/original.mp4",
  "quality_targets": [360, 720, 1080],
  "timestamp": "ISO8601"
}
```

### 4.3 FFmpeg Worker (Go, Kubernetes HPA)

- Consumes from `video.uploaded` Kafka topic (consumer group: `ffmpeg-workers`)
- Transcodes to 360p, 720p, 1080p using FFmpeg binary (shelled out from Go — no CGO)
- Produces HLS output: `master.m3u8` + `.ts` segments per quality level
- Writes output to MinIO HLS bucket: `courses/{course_id}/master.m3u8`
- Publishes `video.processed` Kafka event on completion
- **Scales to zero** when Kafka consumer lag is 0 — major cost saving

### 4.4 Module Metadata Writeback

After transcoding, a Go worker writes back to the LMS Service:

```
hls_url, thumbnail_url, duration_seconds, status = "ready"
```

The course becomes visible to students only when `status = "ready"`.

---

## 5. AWS Video Delivery Layer

### 5.1 Component Map

| AWS Service | Role |
|---|---|
| Route 53 | Domain alias — `lms.yourdomain.com` → CloudFront |
| ACM | Free TLS certificate, auto-renewing, attached to CloudFront |
| CloudFront CDN | HLS delivery with signed URLs (TTL: 4 hours) |
| WAF (optional) | Block unsigned CloudFront requests |

### 5.2 Signed URL Generation (Go)

```go
signer := sign.NewURLSigner(keyID, privateKey)
signedURL, _ := signer.Sign(hlsURL, time.Now().Add(4*time.Hour))
```

The LMS Service generates a signed CloudFront URL when a student requests to play a module. The URL expires after 4 hours — preventing hotlinking or session sharing.

### 5.3 MinIO vs S3

MinIO is kept self-hosted (cheapest option). CloudFront points at the MinIO endpoint as a custom origin. Migration to S3 requires only swapping the Go SDK client — both are S3-compatible.

### 5.4 Pricing (approximate)

- **CloudFront:** ~$0.0085/GB data transfer (India region)
- **Route 53:** $0.50/hosted zone/month
- **ACM TLS:** Free
- **First 1TB/month:** Free under AWS Free Tier

---

## 6. Key Data Models (PostgreSQL)

### 6.1 courses

| Column | Type | Notes |
|---|---|---|
| id | uuid PK | |
| created_by | uuid FK | lms_user_id |
| title | text | |
| description | text | |
| status | enum | pending, processing, ready, archived |
| hls_url | text | CloudFront signed base URL |
| thumbnail_url | text | |
| created_at | timestamptz | |
| updated_at | timestamptz | |

### 6.2 modules

| Column | Type | Notes |
|---|---|---|
| id | uuid PK | |
| course_id | uuid FK | |
| title | text | |
| description | text | |
| duration_seconds | int | |
| order_index | int | Display order within course |

### 6.3 assessment_results

| Column | Type | Notes |
|---|---|---|
| id | uuid PK | |
| lms_user_id | uuid FK | |
| course_id | uuid FK | |
| score | int | 0–100 |
| passed | bool | |
| attempt_number | int | |
| completed_at | timestamptz | |

---

## 7. Analytics & Leaderboard

### 7.1 ClickHouse Events

All user interactions are streamed as events to ClickHouse for fast analytical queries:

- `video_watched` — `{user_id, course_id, module_id, watch_pct, timestamp}`
- `quiz_attempted` — `{user_id, course_id, score, passed, timestamp}`
- `drop_off_at` — `{user_id, module_id, seconds_watched, timestamp}`

### 7.2 Video Performance Metrics

| Metric | Description |
|---|---|
| Avg watch % | Average completion percentage per module |
| Drop-off heatmap | At which second users stop watching |
| Rewatch ratio | How many users replay a module |

These metrics help instructors identify which content is engaging and where learners struggle.

### 7.3 Leaderboard (Redis Sorted Sets)

Only users who have `passed = true` on all required modules of a course are scored.

```
ZADD lb:all <score> <lms_user_id>
ZREVRANK lb:all <lms_user_id>   → position
ZREVRANGE lb:all 0 9            → top 10
```

**Score formula:**

```
score = Σ (quiz_score × course_weight) + completion_bonus − time_penalty
```

All weights are configurable per course by admins.

---

## 8. Support Services

### 8.1 Notification Service

Sends alerts via email, push notification, and in-app banner for:
- Course completion
- New course published
- Quiz result
- Leaderboard position change

Consumes from Kafka topics to remain decoupled from core services.

### 8.2 Video Analytics Engine

Aggregates ClickHouse data into pre-computed dashboards for instructors:
- Which videos perform best (high completion, low drop-off)
- Which modules need rework
- Cohort comparison across departments

---

## 9. Scalability & Budget Notes

| Concern | Approach |
|---|---|
| FFmpeg workers | Kubernetes HPA on Kafka consumer lag — scale to 0 when idle |
| Object storage | MinIO self-hosted; swap to S3 when managed storage is justified |
| PostgreSQL | Single instance + read replicas up to ~50k users; Citus for sharding beyond that |
| Go services | Single binary, low memory — 3 small VMs covers most of the stack |
| Redis | Managed Redis (AWS ElastiCache) or self-hosted — both work |
| ClickHouse | Single-node handles millions of events/day cheaply |

---

## 10. Go Technology Choices

| Component | Library / Tool |
|---|---|
| HTTP layer | `fiber`
| Kafka client | `watermill` |
| PostgreSQL queries | `sqlc` (type-safe, generated) |
| FFmpeg | Shell out to binary — no CGO |
| Concurrent transcode | `errgroup` |
| CloudFront signing | AWS SDK v2 `cloudfront/sign` |
| JWT | `golang-jwt/jwt` |

---

*Document prepared by SmartFM Engineering. Internal use only.*
