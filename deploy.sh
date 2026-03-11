#!/bin/bash

# ==============================================================================
# Script Otomasi Deployment Payment Service
# Cara Pakai: ./deploy.sh
# ==============================================================================

# Nama aplikasi (sesuaikan dengan nama binary Anda)
APP_NAME="payment_service"
# Nama service systemd Anda
SERVICE_NAME="payment-api.service"

echo "🚀 Memulai proses deployment..."

# 1. Tarik kode terbaru dari GitHub
echo "📥 Menarik kode terbaru dari Git..."
git pull origin main

# 2. Instal/Update dependencies
echo "📦 Memperbarui dependencies Go..."
go mod tidy

# 3. Build ulang binary
echo "🏗️  Membangun ulang aplikasi..."
go build -o $APP_NAME cmd/main.go

if [ $? -ne 0 ]; then
    echo "❌ Build Gagal! Deployment dihentikan."
    exit 1
fi

# 4. Berikan izin eksekusi pada binary baru
chmod +x $APP_NAME

# 5. Restart service untuk menerapkan perubahan
echo "🔄 Merestart service $SERVICE_NAME..."
sudo systemctl restart $SERVICE_NAME

# 6. Cek status service
echo "⏳ Mengecek status aplikasi..."
sleep 2
sudo systemctl status $SERVICE_NAME --no-pager

echo "✅ Deployment selesai dengan sukses!"
