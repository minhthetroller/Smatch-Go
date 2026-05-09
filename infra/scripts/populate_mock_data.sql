-- Populate mock data for Smatch Badminton admin/owner features
-- Run with: psql $DATABASE_URL -f infra/scripts/populate_mock_data.sql

-- Clear existing mock data first (optional - uncomment if you want a clean slate)
-- DELETE FROM admin_audit_logs;
-- DELETE FROM payments;
-- DELETE FROM bookings;
-- DELETE FROM match_players;
-- DELETE FROM matches;
-- DELETE FROM pricing_rules;
-- DELETE FROM sub_court_closures;
-- DELETE FROM sub_courts;
-- DELETE FROM business_profile_applications;
-- DELETE FROM courts WHERE owner_user_id IS NOT NULL;
-- DELETE FROM users WHERE firebase_uid LIKE 'mock_%';

WITH
admin_user AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  VALUES ('mock_admin_uid_1', 'admin@smatch.com', 'admin', 'password', '{admin}', 'System', 'Admin', '+84111111111', 'Hà Nội', NOW() - INTERVAL '30 days')
  ON CONFLICT (firebase_uid) DO UPDATE SET roles = '{admin}'
  RETURNING id
),
owner1 AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  VALUES ('mock_owner_uid_1', 'owner1@smatch.com', 'owner1', 'google', '{user,court_owner}', 'Nguyễn', 'Văn A', '+84222222222', 'Hà Nội', NOW() - INTERVAL '25 days')
  ON CONFLICT (firebase_uid) DO UPDATE SET roles = '{user,court_owner}'
  RETURNING id
),
owner2 AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  VALUES ('mock_owner_uid_2', 'owner2@smatch.com', 'owner2', 'google', '{user,court_owner}', 'Trần', 'Thị B', '+84333333333', 'Hồ Chí Minh', NOW() - INTERVAL '20 days')
  ON CONFLICT (firebase_uid) DO UPDATE SET roles = '{user,court_owner}'
  RETURNING id
),
owner3 AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  VALUES ('mock_owner_uid_3', 'owner3@smatch.com', 'owner3', 'password', '{user}', 'Lê', 'Văn C', '+84444444444', 'Hà Nội', NOW() - INTERVAL '15 days')
  ON CONFLICT (firebase_uid) DO NOTHING
  RETURNING id
),
owner4 AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  VALUES ('mock_owner_uid_4', 'owner4@smatch.com', 'owner4', 'google', '{user}', 'Phạm', 'Thị D', '+84555555555', 'Đà Nẵng', NOW() - INTERVAL '10 days')
  ON CONFLICT (firebase_uid) DO NOTHING
  RETURNING id
),
owner5 AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  VALUES ('mock_owner_uid_5', 'owner5@smatch.com', 'owner5', 'password', '{user}', 'Hoàng', 'Văn E', '+84666666666', 'Hà Nội', NOW() - INTERVAL '5 days')
  ON CONFLICT (firebase_uid) DO NOTHING
  RETURNING id
),
regular_users AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  SELECT
    'mock_user_uid_' || gs,
    'user' || gs || '@smatch.com',
    'user' || gs,
    CASE WHEN gs % 2 = 0 THEN 'google' ELSE 'password' END,
    '{user}',
    CASE WHEN gs % 3 = 0 THEN 'An' WHEN gs % 3 = 1 THEN 'Bình' ELSE 'Cường' END,
    CASE WHEN gs % 3 = 0 THEN 'Nguyễn' WHEN gs % 3 = 1 THEN 'Trần' ELSE 'Lê' END,
    '+84' || lpad(gs::text, 9, '0'),
    CASE WHEN gs % 2 = 0 THEN 'Hà Nội' ELSE 'Hồ Chí Minh' END,
    NOW() - INTERVAL '1 hour' * gs
  FROM generate_series(1, 50) AS gs
  ON CONFLICT (firebase_uid) DO NOTHING
  RETURNING id
),
court1 AS (
  INSERT INTO courts (name, description, phone_numbers, address_street, address_ward, address_district, address_city, details, opening_hours, location, owner_user_id, created_at)
  SELECT
    'Smash Court Center',
    'Professional badminton courts with AC and lighting',
    ARRAY['+84222222222'],
    '123 Lê Duẩn',
    'Phường Bến Nghé',
    'Quận 1',
    'Hồ Chí Minh',
    '{"amenities": ["parking", "ac", "pro_shop"]}'::jsonb,
    '{"monday": {"open": "06:00", "close": "22:00"}}'::jsonb,
    NULL,
    owner1.id,
    NOW() - INTERVAL '24 days'
  FROM owner1
  RETURNING id
),
court2 AS (
  INSERT INTO courts (name, description, phone_numbers, address_street, address_ward, address_district, address_city, details, opening_hours, location, owner_user_id, created_at)
  SELECT
    'Shuttle Hub',
    'Community badminton facility with 8 courts',
    ARRAY['+84333333333'],
    '456 Nguyễn Trãi',
    'Phường Thanh Xuân Trung',
    'Quận Thanh Xuân',
    'Hà Nội',
    '{"amenities": ["parking", "cafe"]}'::jsonb,
    '{"monday": {"open": "05:30", "close": "23:00"}}'::jsonb,
    NULL,
    owner2.id,
    NOW() - INTERVAL '19 days'
  FROM owner2
  RETURNING id
),
court3 AS (
  INSERT INTO courts (name, description, phone_numbers, address_street, address_ward, address_district, address_city, details, opening_hours, location, owner_user_id, created_at)
  SELECT
    'Birdie Arena',
    'Premium courts with wooden flooring',
    ARRAY['+84444444444'],
    '789 Lý Thường Kiệt',
    'Phường 7',
    'Quận Tân Bình',
    'Hồ Chí Minh',
    '{"amenities": ["ac", "locker_room"]}'::jsonb,
    '{"monday": {"open": "06:00", "close": "21:00"}}'::jsonb,
    NULL,
    owner1.id,
    NOW() - INTERVAL '22 days'
  FROM owner1
  RETURNING id
),
subcourts AS (
  INSERT INTO sub_courts (court_id, name, description, is_active)
  SELECT c.id, 'Court ' || s, 'Standard court', true
  FROM (SELECT id FROM court1 UNION ALL SELECT id FROM court2 UNION ALL SELECT id FROM court3) AS c(id)
  CROSS JOIN generate_series(1, 4) AS s
  RETURNING id, court_id
),
pricing AS (
  INSERT INTO pricing_rules (court_id, name, day_type, start_time, end_time, price_per_hour, is_active)
  SELECT c.id, 'Weekday Morning', 'weekday', '06:00'::time, '12:00'::time, 80000, true
  FROM (SELECT id FROM court1 UNION ALL SELECT id FROM court2 UNION ALL SELECT id FROM court3) AS c(id)
  UNION ALL
  SELECT c.id, 'Weekday Afternoon', 'weekday', '12:00'::time, '18:00'::time, 120000, true
  FROM (SELECT id FROM court1 UNION ALL SELECT id FROM court2 UNION ALL SELECT id FROM court3) AS c(id)
  UNION ALL
  SELECT c.id, 'Weekday Evening', 'weekday', '18:00'::time, '22:00'::time, 180000, true
  FROM (SELECT id FROM court1 UNION ALL SELECT id FROM court2 UNION ALL SELECT id FROM court3) AS c(id)
  UNION ALL
  SELECT c.id, 'Weekend', 'weekend', '06:00'::time, '22:00'::time, 200000, true
  FROM (SELECT id FROM court1 UNION ALL SELECT id FROM court2 UNION ALL SELECT id FROM court3) AS c(id)
),
bp_approved AS (
  INSERT INTO business_profile_applications (
    user_id, legal_representative_name, personal_id_number,
    personal_id_front_image_url, personal_id_back_image_url,
    business_registration_cert_url, sports_business_eligibility_cert_url,
    fire_safety_cert_url, tax_id_number, proof_of_address_url,
    bank_account_number, bank_name, bank_branch, bank_account_holder_name,
    operational_specs, status, admin_notes, submitted_at, reviewed_at, reviewed_by
  )
  SELECT
    owner1.id,
    'Nguyễn Văn A',
    '001099012345',
    'https://placehold.co/600x400?text=ID+Front',
    'https://placehold.co/600x400?text=ID+Back',
    'https://placehold.co/600x400?text=Business+Reg',
    'https://placehold.co/600x400?text=Sports+Elig',
    'https://placehold.co/600x400?text=Fire+Safety',
    '0312345678',
    'https://placehold.co/600x400?text=Address',
    '1234567890',
    'Vietcombank',
    'Hà Nội',
    'Nguyễn Văn A',
    '{"subcourt_count": 4, "surface_type": "wooden", "operating_hours": {"open": "06:00", "close": "22:00"}, "base_pricing": [{"day_type": "weekday", "start_time": "06:00", "end_time": "12:00", "price_per_hour": 80000}, {"day_type": "weekday", "start_time": "12:00", "end_time": "18:00", "price_per_hour": 120000}, {"day_type": "weekday", "start_time": "18:00", "end_time": "22:00", "price_per_hour": 180000}, {"day_type": "weekend", "start_time": "06:00", "end_time": "22:00", "price_per_hour": 200000}]}'::jsonb,
    'approved',
    NULL,
    NOW() - INTERVAL '24 days',
    NOW() - INTERVAL '23 days',
    admin_user.id
  FROM owner1, admin_user
  ON CONFLICT (user_id) DO NOTHING
),
bp_approved2 AS (
  INSERT INTO business_profile_applications (
    user_id, legal_representative_name, personal_id_number,
    personal_id_front_image_url, personal_id_back_image_url,
    business_registration_cert_url, sports_business_eligibility_cert_url,
    fire_safety_cert_url, tax_id_number, proof_of_address_url,
    bank_account_number, bank_name, bank_branch, bank_account_holder_name,
    operational_specs, status, admin_notes, submitted_at, reviewed_at, reviewed_by
  )
  SELECT
    owner2.id,
    'Trần Thị B',
    '001099054321',
    'https://placehold.co/600x400?text=ID+Front',
    'https://placehold.co/600x400?text=ID+Back',
    'https://placehold.co/600x400?text=Business+Reg',
    'https://placehold.co/600x400?text=Sports+Elig',
    'https://placehold.co/600x400?text=Fire+Safety',
    '0312345679',
    'https://placehold.co/600x400?text=Address',
    '0987654321',
    'Techcombank',
    'Hồ Chí Minh',
    'Trần Thị B',
    '{"subcourt_count": 4, "surface_type": "synthetic", "operating_hours": {"open": "05:30", "close": "23:00"}, "base_pricing": [{"day_type": "weekday", "start_time": "05:30", "end_time": "12:00", "price_per_hour": 70000}, {"day_type": "weekday", "start_time": "12:00", "end_time": "18:00", "price_per_hour": 100000}, {"day_type": "weekday", "start_time": "18:00", "end_time": "23:00", "price_per_hour": 150000}]}'::jsonb,
    'approved',
    NULL,
    NOW() - INTERVAL '19 days',
    NOW() - INTERVAL '18 days',
    admin_user.id
  FROM owner2, admin_user
  ON CONFLICT (user_id) DO NOTHING
),
bp_pending AS (
  INSERT INTO business_profile_applications (
    user_id, legal_representative_name, personal_id_number,
    personal_id_front_image_url, personal_id_back_image_url,
    business_registration_cert_url, sports_business_eligibility_cert_url,
    fire_safety_cert_url, tax_id_number, proof_of_address_url,
    bank_account_number, bank_name, bank_branch, bank_account_holder_name,
    operational_specs, status, submitted_at
  )
  SELECT
    owner3.id,
    'Lê Văn C',
    '001099078901',
    'https://placehold.co/600x400?text=ID+Front',
    'https://placehold.co/600x400?text=ID+Back',
    'https://placehold.co/600x400?text=Business+Reg',
    'https://placehold.co/600x400?text=Sports+Elig',
    'https://placehold.co/600x400?text=Fire+Safety',
    '0312345680',
    'https://placehold.co/600x400?text=Address',
    '1122334455',
    'MB Bank',
    'Hà Nội',
    'Lê Văn C',
    '{"subcourt_count": 2, "surface_type": "synthetic", "operating_hours": {"open": "06:00", "close": "21:00"}, "base_pricing": [{"day_type": "weekday", "start_time": "06:00", "end_time": "21:00", "price_per_hour": 90000}]}'::jsonb,
    'pending',
    NOW() - INTERVAL '3 days'
  FROM owner3
  ON CONFLICT (user_id) DO NOTHING
),
bp_resubmit AS (
  INSERT INTO business_profile_applications (
    user_id, legal_representative_name, personal_id_number,
    personal_id_front_image_url, personal_id_back_image_url,
    business_registration_cert_url, sports_business_eligibility_cert_url,
    fire_safety_cert_url, tax_id_number, proof_of_address_url,
    bank_account_number, bank_name, bank_branch, bank_account_holder_name,
    operational_specs, status, admin_notes, submitted_at, reviewed_at, reviewed_by
  )
  SELECT
    owner4.id,
    'Phạm Thị D',
    '001099011223',
    'https://placehold.co/600x400?text=ID+Front',
    'https://placehold.co/600x400?text=ID+Back',
    'https://placehold.co/600x400?text=Business+Reg',
    'https://placehold.co/600x400?text=Sports+Elig',
    NULL,
    '0312345681',
    'https://placehold.co/600x400?text=Address',
    '5566778899',
    'ACB',
    'Đà Nẵng',
    'Phạm Thị D',
    '{"subcourt_count": 3, "surface_type": "wooden", "operating_hours": {"open": "06:00", "close": "22:00"}, "base_pricing": [{"day_type": "weekday", "start_time": "06:00", "end_time": "22:00", "price_per_hour": 100000}]}'::jsonb,
    'resubmit_requested',
    'Fire safety certificate is missing. Please upload the certificate and resubmit. Also ensure the business registration certificate is clearly scanned.',
    NOW() - INTERVAL '8 days',
    NOW() - INTERVAL '6 days',
    admin_user.id
  FROM owner4, admin_user
  ON CONFLICT (user_id) DO NOTHING
),
bp_rejected_notes AS (
  INSERT INTO business_profile_applications (
    user_id, legal_representative_name, personal_id_number,
    personal_id_front_image_url, personal_id_back_image_url,
    business_registration_cert_url, sports_business_eligibility_cert_url,
    fire_safety_cert_url, tax_id_number, proof_of_address_url,
    bank_account_number, bank_name, bank_branch, bank_account_holder_name,
    operational_specs, status, admin_notes, submitted_at, reviewed_at, reviewed_by
  )
  SELECT
    owner5.id,
    'Hoàng Văn E',
    '001099033445',
    'https://placehold.co/600x400?text=ID+Front',
    'https://placehold.co/600x400?text=ID+Back',
    'https://placehold.co/600x400?text=Business+Reg',
    'https://placehold.co/600x400?text=Sports+Elig',
    'https://placehold.co/600x400?text=Fire+Safety',
    '0312345682',
    'https://placehold.co/600x400?text=Address',
    '9988776655',
    'VPBank',
    'Hà Nội',
    'Hoàng Văn E',
    '{"subcourt_count": 5, "surface_type": "synthetic", "operating_hours": {"open": "05:00", "close": "23:00"}, "base_pricing": [{"day_type": "weekday", "start_time": "05:00", "end_time": "23:00", "price_per_hour": 85000}]}'::jsonb,
    'rejected',
    'The tax ID number does not match the business registration. Please correct the tax ID and ensure all documents are consistent before resubmitting.',
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '4 days',
    admin_user.id
  FROM owner5, admin_user
  ON CONFLICT (user_id) DO NOTHING
),
bp_rejected_illegal AS (
  INSERT INTO users (firebase_uid, email, username, provider, roles, first_name, last_name, phone_number, address_city, created_at)
  VALUES ('mock_owner_uid_6', 'owner6@smatch.com', 'owner6', 'google', '{user}', 'Vũ', 'Thị F', '+84777777777', 'Hà Nội', NOW() - INTERVAL '12 days')
  ON CONFLICT (firebase_uid) DO NOTHING
  RETURNING id
),
bp_rejected_illegal_insert AS (
  INSERT INTO business_profile_applications (
    user_id, legal_representative_name, personal_id_number,
    personal_id_front_image_url, personal_id_back_image_url,
    business_registration_cert_url, sports_business_eligibility_cert_url,
    fire_safety_cert_url, tax_id_number, proof_of_address_url,
    bank_account_number, bank_name, bank_branch, bank_account_holder_name,
    operational_specs, status, admin_notes, submitted_at, reviewed_at, reviewed_by
  )
  SELECT
    bp_rejected_illegal.id,
    'Vũ Thị F',
    '001099055667',
    'https://placehold.co/600x400?text=ID+Front',
    'https://placehold.co/600x400?text=ID+Back',
    'https://placehold.co/600x400?text=Business+Reg',
    'https://placehold.co/600x400?text=Sports+Elig',
    'https://placehold.co/600x400?text=Fire+Safety',
    '0312345683',
    'https://placehold.co/600x400?text=Address',
    '8877665544',
    'Sacombank',
    'Hà Nội',
    'Vũ Thị F',
    '{"subcourt_count": 6, "surface_type": "wooden", "operating_hours": {"open": "06:00", "close": "22:00"}, "base_pricing": [{"day_type": "weekday", "start_time": "06:00", "end_time": "22:00", "price_per_hour": 95000}]}'::jsonb,
    'rejected',
    NULL,
    NOW() - INTERVAL '11 days',
    NOW() - INTERVAL '10 days',
    admin_user.id
  FROM bp_rejected_illegal, admin_user
  ON CONFLICT (user_id) DO NOTHING
),
bookings_data AS (
  INSERT INTO bookings (sub_court_id, user_id, guest_name, guest_phone, date, start_time, end_time, total_price, status, created_at)
  SELECT
    sc.id,
    u.id,
    NULL,
    NULL,
    CURRENT_DATE - (random() * 30)::int,
    '06:00'::time + (random() * 12 || ' hours')::interval,
    '18:00'::time,
    (random() * 200000 + 50000)::int,
    (ARRAY['pending', 'confirmed', 'completed', 'cancelled'])[1 + (random() * 3)::int],
    NOW() - INTERVAL '1 hour' * random() * 720
  FROM subcourts sc
  CROSS JOIN LATERAL (
    SELECT id FROM users WHERE roles @> '{user}' ORDER BY random() LIMIT 1
  ) u
  CROSS JOIN generate_series(1, 3)
  WHERE sc.court_id IN (SELECT id FROM court1 UNION ALL SELECT id FROM court2 UNION ALL SELECT id FROM court3)
),
payments_data AS (
  INSERT INTO payments (booking_id, app_trans_id, amount, status, created_at)
  SELECT
    b.id,
    'APP' || lpad((row_number() OVER ())::text, 10, '0'),
    b.total_price,
    CASE WHEN random() > 0.2 THEN 'success' ELSE 'failed' END,
    b.created_at
  FROM bookings b
  WHERE b.status IN ('confirmed', 'completed')
),
audit_logs AS (
  INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, details, created_at)
  SELECT
    admin_user.id,
    'review_business_profile:approve',
    'business_profile',
    bp.id,
    '{"action": "approve", "admin_notes": null}'::jsonb,
    bp.reviewed_at
  FROM business_profile_applications bp
  CROSS JOIN admin_user
  WHERE bp.status = 'approved' AND bp.reviewed_by = admin_user.id
)
SELECT 'Mock data populated successfully' AS result;
