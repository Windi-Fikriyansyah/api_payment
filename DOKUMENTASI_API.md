# 📚 Dokumentasi API LinkBayar

Terakhir diperbarui: 17 Maret 2026

## 1. Pendahuluan
Dokumentasi ini ditujukan untuk merchant yang ingin mengintegrasikan sistem pembayaran **LinkBayar** ke dalam website atau aplikasi mereka.

Dengan API ini, Anda dapat:
- Membuat transaksi pembayaran (QRIS, Virtual Account, Retail)
- Menerima notifikasi otomatis saat pembayaran berhasil (Webhook)
- Integrasi mudah dengan Checkout Page (**Integrasi Via URL**)
- Membatalkan transaksi yang belum dibayar
- Mengecek status/detail transaksi

---

## 2. Persiapan
Sebelum mulai menggunakan API, pastikan Anda telah:
1. Mendaftar akun di dashboard **LinkBayar**.
2. Membuat project baru.
3. Mencatat **API Key** dan **Slug** dari halaman detail project.
4. Mengatur **Webhook URL** di pengaturan project.

**Informasi Penting:**
- **Base URL**: `https://app.linkbayar.my.id`
- **API Key**: Tersedia di dashboard project.
- **Slug**: Nama unik project Anda (digunakan untuk Via URL).

---

## 3. Autentikasi
Semua request API memerlukan API Key untuk autentikasi. Kirimkan melalui header:
`X-API-Key: api_key_anda`

---

## 4. Daftar API

### A. Membuat Transaksi (Direct API)
Gunakan API ini jika Anda ingin membuat halaman checkout sendiri.

- **Method**: `POST`
- **URL**: `https://app.linkbayar.my.id/api/transactioncreate/{method}`
- **Body (JSON)**:
```json
{
    "project": "nama_project_anda",
    "order_id": "INV123",
    "amount": 50000,
    "api_key": "api_key_anda"
}
```

### B. Integrasi Via URL (Checkout Page) 🚀
Metode ini paling aman dan profesional. URL yang dihasilkan sangat pendek dan parameter aslinya tersembunyi.

**Langkah 1: Buat Sesi Pembayaran (Server-to-Server)**
Merchant memanggil API ini dari backend untuk mendapatkan token pembayaran.
- **Method**: `POST`
- **URL**: `https://app.linkbayar.my.id/api/checkout-session`
- **Headers**: `X-API-Key: api_key_anda`
- **Body (JSON)**:
```json
{
    "amount": 50000,
    "order_id": "INV-123",
    "redirect_url": "https://tokoanda.com/success"
}
```
**Response Sukses:**
```json
{
    "payment_url": "https://app.linkbayar.my.id/pay/tokoonline/e8ff1622749f6a48...",
    "order_id": "INV-123",
    "amount": 50000
}
```

**Langkah 2: Arahkan Pelanggan**
Cukup arahkan pelanggan ke `payment_url` yang Anda dapatkan dari Langkah 1. URL ini berlaku selama 1 jam.

**Keuntungan:**
- URL sangat bersih dan pendek.
- Nominal dan Order ID tidak bisa diubah oleh user.
- Parameter sensitif tidak terlihat di browser.

### C. Cek Status/Detail Transaksi
Gunakan API ini untuk mendapatkan detail lengkap transaksi.
- **Method**: `GET`
- **URL**: `https://app.linkbayar.my.id/api/transactiondetail?order_id=INV-123`
- **Headers**: `X-API-Key: api_key_anda`

### D. Batalkan Transaksi
Membatalkan transaksi yang masih berstatus `pending`.
- **Method**: `POST`
- **URL**: `https://app.linkbayar.my.id/api/transactioncancel`
- **Headers**: `X-API-Key: api_key_anda`
- **Body (JSON)**:
```json
{
    "project": "nama_project_anda",
    "order_id": "INV-123",
    "amount": 50000
}
```

---

## 5. Webhook (Notifikasi Otomatis)
Sistem kami akan mengirimkan POST ke URL Webhook Anda saat status transaksi berubah (berhasil/expired/batal).

**Payload (JSON):**
```json
{
    "amount": 50000,
    "fee": 2500,
    "net_amount": 47500,
    "order_id": "ORD-001",
    "project": "tokoonline",
    "status": "success",
    "payment_method": "qris",
    "completed_at": "2026-03-17T14:40:00Z"
}
```

---

## 6. Testing & Sandbox
Pastikan project Anda dalam mode **Sandbox** saat melakukan pengujian. Anda dapat mensimulasikan pembayaran sukses melalui API simulation:
- **URL**: `POST /api/paymentsimulation`
- **Body**: Sama dengan body request create transaction.

---

## 7. Bantuan
Jika ada kendala, hubungi tim teknis kami:
- **Email**: `support@linkbayar.my.id`
- **Website**: [linkbayar.my.id](https://app.linkbayar.my.id)
