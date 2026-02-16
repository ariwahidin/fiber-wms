#!/usr/bin/env python3
import sys
import json
import re
import pdfplumber

def extract_field(text, pattern, group=1):
    """Extract field using regex pattern"""
    match = re.search(pattern, text, re.IGNORECASE | re.MULTILINE)
    return match.group(group).strip() if match else ""

def parse_consignment_memo(pdf_path):
    """
    Parse Goods Consignment Memo PDF and extract key information
    """
    try:
        with pdfplumber.open(pdf_path) as pdf:
            # Extract text from all pages
            full_text = ""
            for page in pdf.pages:
                full_text += page.extract_text() + "\n"
            
            # Extract fields using regex patterns
            data = {}
            
            # Document Number (No : PT26020601 or PT26020609)
            doc_no_pattern = r'No\s*:\s*(PT\d+)'
            data['docNo'] = extract_field(full_text, doc_no_pattern)
            
            # Vendor / Goods Owner
            vendor_pattern = r'Vendor\s*/\s*Goods\s+Owner\s*[:：]\s*(.+?)(?:\n|Type of Goods)'
            vendor = extract_field(full_text, vendor_pattern)
            # Clean up vendor name
            if vendor:
                vendor = re.sub(r'\s+', ' ', vendor).strip()
            data['vendor'] = vendor
            
            # Type of Goods / Item Name
            item_pattern = r'Type\s+of\s+Goods\s*[:：]\s*(.+?)(?:\n|Quantity)'
            item_name = extract_field(full_text, item_pattern)
            if item_name:
                item_name = re.sub(r'\s+', ' ', item_name).strip()
            data['itemName'] = item_name
            
            # SKU (from Batch No / SKU line)
            sku_pattern = r'Batch\s+No\s*/\s*SKU\s*[:：]\s*(?:RE\d+\s*/\s*)?(\d+)'
            sku = extract_field(full_text, sku_pattern)
            # Try alternative pattern if not found
            if not sku:
                # Handle OCR errors like "3OO3 1 1 82"
                alt_sku_pattern = r'Batch\s+No\s*/\s*SKU\s*[:：].+?([0-9OIl\s]{8,})'
                sku_raw = extract_field(full_text, alt_sku_pattern)
                if sku_raw:
                    # Clean OCR errors: O->0, I->1, l->1, remove spaces
                    sku = sku_raw.replace('O', '0').replace('I', '1').replace('l', '1').replace(' ', '')
                    # Take only digits
                    sku = ''.join(c for c in sku if c.isdigit())
            data['sku'] = sku
            
            # Batch Number
            # batch_pattern = r'Batch\s+No\s*/\s*SKU\s*[:：]\s*(RE202\d+)'
            # batch = extract_field(full_text, batch_pattern)
            # # Try to handle OCR errors
            # if not batch:
            #     alt_batch_pattern = r'Batch\s+No\s*/\s*SKU\s*[:：]\s*\d+\s*/\s*([RE0-9OIl\s]+?)\s*[/I]'
            #     batch_raw = extract_field(full_text, alt_batch_pattern)
            #     if batch_raw:
            #         # Clean OCR errors
            #         batch = batch_raw.replace('O', '0').replace('I', '1').replace('l', '1').replace(' ', '')
            #         # Ensure it starts with RE
            #         if not batch.startswith('RE'):
            #             batch = 'RE' + ''.join(c for c in batch if c.isdigit())
            # data['batchNo'] = batch
            
            # 1. Cari batch number langsung (posisi bebas)
            batch_pattern = r'\bRE202[0-9OIl]{3,}\b'
            match = re.search(batch_pattern, full_text)

            if match:
                # 2. Bersihin hasil OCR
                batch = (
                    match.group()
                    .replace('O', '0')
                    .replace('I', '1')
                    .replace('l', '1')
                )

            data['batchNo'] = batch
            
            # Quantity (extract number from "180 Box (1 Carton)" or "360 Box (2 Carton)")
            # qty_pattern = r'Quantity\s*[:：]\s*(\d+)\s*Box'
            # qty_str = extract_field(full_text, qty_pattern)
            qty_pattern = r'Quantity\s*[:：]\s*(\d+)\s*(Box|Unit)'
            qty_str = extract_field(full_text, qty_pattern)
            data['qty'] = qty_str if qty_str else "0"
            
            # Storage Location
            location_pattern = r'Storage\s+Location\s*[:：]\s*(.+?)(?:\n\n|Please be advised)'
            location = extract_field(full_text, location_pattern)
            if location:
                # Clean up location - remove extra whitespace and newlines
                location = re.sub(r'\s+', ' ', location).strip()
                # Remove any trailing periods or commas
                location = location.rstrip('.,')
                # Handle OCR errors in location
                location = location.replace("'1 JAI'ARTA PU", "Jakarta Pusat")
            # If still empty, try simpler pattern
            if not location:
                simple_loc_pattern = r'Storage\s+Location\s*[:：]\s*([^\n]+)'
                location = extract_field(full_text, simple_loc_pattern)
                if location:
                    location = re.sub(r'\s+', ' ', location).strip()
            data['location'] = location
            
            # Consignment Period
            period_pattern = r"Consignment\s+Period\s*[:：']\s*(.+?)(?:\n|Storage)"
            period = extract_field(full_text, period_pattern)
            if period:
                period = re.sub(r'\s+', ' ', period).strip()
                # Handle OCR errors like "F eb - Jwrc 2026"
                period = period.replace('Jwrc', 'June').replace('F eb', 'Feb')
            data['consignmentPeriod'] = period
            
            return {
                'success': True,
                'data': data
            }
            
    except Exception as e:
        return {
            'success': False,
            'error': str(e)
        }

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(json.dumps({
            'success': False,
            'error': 'No PDF file path provided'
        }))
        sys.exit(1)
    
    pdf_path = sys.argv[1]
    result = parse_consignment_memo(pdf_path)
    print(json.dumps(result, ensure_ascii=False))