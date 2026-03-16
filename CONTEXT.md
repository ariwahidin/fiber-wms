# CONTEXT.md — WMS Picking Feature

## Stack
- Backend: Go + Fiber
- Frontend: Next.js + TypeScript
- Database: SQL Server
- Pattern: Handler → Service → Repository

## Fitur yang Dibangun
Outbound Picking — operator scan barcode/QR produk untuk memenuhi picking order.

## Alur Picking
1. Operator buka halaman picking, pilih shipment/order
2. Sistem tampilkan list item yang harus dipick
3. Operator scan QR/barcode per unit atau carton
4. Sistem validasi: SKU cocok, qty belum over-pick, stok tersedia
5. Progress terupdate realtime per item
6. Setelah semua item complete → picking order bisa di-confirm

## QR Format 
- Parenthesis-style: (01)06933257941045(10)BATCH(17)260312(30)20
- 12-segment dash-separated
- Label type: UNIT | CARTON
- Fungsi parser: `parseQRCode` — jangan dibuat ulang, pakai yang sudah ada

## Struktur Data Utama
- PickingOrder: id, shipment_id, status, created_at
- PickingOrderItem: id, picking_order_id, sku, qty_required, qty_picked
- PickingLog: id, picking_order_item_id, scanned_value, label_type, scanned_at

## Konvensi Frontend
- Komponen: `OutboundPickingPage.tsx`
- Scan handler: debounce 300ms, cek duplikat scan
- Over-pick harus ditolak dengan pesan jelas
- Progress tampil per item (qty_picked / qty_required)
- Error: toast notification, bukan alert/modal
- Loading state saat hit API

## SQL Server — Hal Penting
- Gunakan transaction untuk update qty_picked + insert PickingLog sekaligus
- Cek over-pick di level query, bukan hanya di aplikasi
- Gunakan `GETDATE()` untuk timestamp

## Batasan untuk AI
- Jangan ubah `parseQRCode` yang sudah ada
- Jangan ubah file di luar scope picking feature
- Jangan tambah library/dependency baru tanpa konfirmasi
- Jangan rename fungsi atau tipe yang sudah ada
- Kalau tidak yakin dengan struktur tabel, tanya dulu sebelum nulis query