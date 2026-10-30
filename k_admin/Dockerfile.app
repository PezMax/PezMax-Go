# KAdmin（PezMax-Go 后端）应用镜像：内置 LibreOffice，支持 doc/docx/ppt/pptx → PDF 转档。
# 一般不直接构建，由根目录 docker-compose.yml 的 kadmin 服务（profiles: ["app"]）引用：
#   docker compose --profile app build kadmin
#   docker compose --profile app up -d kadmin

# ---------- 构建阶段 ----------
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/k_admin .

# ---------- 运行阶段 ----------
# writer 负责 doc/docx，impress 负责 ppt/pptx；Noto CJK 保证中文文档转档不缺字。
FROM debian:bookworm-slim
ARG DEBIAN_MIRROR=
RUN if [ -n "$DEBIAN_MIRROR" ]; then \
        sed -i "s|http://deb.debian.org|${DEBIAN_MIRROR}|g" /etc/apt/sources.list.d/debian.sources; \
    fi \
    && env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy \
        apt-get -o Acquire::Retries=5 update \
    && env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy \
        apt-get -o Acquire::Retries=5 install -y --no-install-recommends \
        ca-certificates \
        libreoffice-writer \
        libreoffice-impress \
        fonts-dejavu-core \
        fonts-noto-cjk \
        tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/k_admin ./k_admin
# 默认敏感词词表随镜像分发（运行时的 data/system_config.json 与它同目录）。
# 线上要改词表内容：把 security.sensitive_words_path（或 KADMIN_SENSITIVE_WORDS_PATH）
# 指到挂载卷里的文件，改动热加载，不需要重建镜像。
COPY --from=build /src/data/sensitive_words.txt ./data/sensitive_words.txt

ENV TZ=Asia/Shanghai \
    KADMIN_APP_PORT=9033 \
    KADMIN_SOFFICE_BIN=soffice \
    KADMIN_CONVERT_TIMEOUT=2m \
    KADMIN_CONVERT_CONCURRENCY=2

EXPOSE 9033
# 上传/静态文件与 soffice 用户目录均需可写
VOLUME ["/app/uploads", "/tmp"]

ENTRYPOINT ["./k_admin"]
