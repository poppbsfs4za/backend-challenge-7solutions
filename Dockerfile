# ---------- build stage ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# copy go.mod/go.sum ก่อน เพื่อให้ layer นี้ถูก cache
# ถ้าโค้ดเปลี่ยนแต่ dependency ไม่เปลี่ยน จะไม่ต้องโหลดใหม่
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 ทำให้ได้ binary แบบ static ไม่ต้องพึ่ง libc
# -ldflags "-s -w" ตัด debug symbol ออก ลดขนาดไฟล์
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/api ./cmd/api

# ---------- runtime stage ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /app/api .

# รันด้วย non-root user
USER appuser

EXPOSE 8080
ENTRYPOINT ["/app/api"]