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
Cara tercepat tanpa coding backend berat. Cukup arahkan pelanggan ke URL kami.

- **Method**: `GET`
- **URL**: `https://app.linkbayar.my.id/pay/{slug}/{amount}`
- **Query Parameters**:
    - `order_id` (wajib): ID pesanan unik Anda.
    - `redirect` (opsional): URL tujuan setelah bayar sukses.

**Contoh Link:**
`https://app.linkbayar.my.id/pay/tokoonline/50000?order_id=ORD-001&redirect=https://tokoanda.com/success`

---

## 5. Webhook (Notifikasi Otomatis)
Sistem kami akan mengirimkan POST ke URL Webhook Anda saat pembayaran berhasil.

**Payload (JSON):**
```json
{
    "amount": 50000,
    "fee": 0,
    "net_amount": 50000,
    "order_id": "ORD-001",
    "project": "tokoonline",
    "status": "success",
    "payment_method": "qris",
    "completed_at": "2026-03-17T14:40:00Z"
}
```

---

## 6. Bantuan
Jika ada kendala, hubungi kami di:
- **Email**: `support@linkbayar.my.id`
- **Website**: [linkbayar.my.id](https://app.linkbayar.my.id)
