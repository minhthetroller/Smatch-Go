-- Seed courts for staging/infrastructure smoke tests.
-- Fixed IDs keep this migration idempotent across repeated runs.

INSERT INTO "courts" (
    "id",
    "name",
    "description",
    "phone_numbers",
    "address_street",
    "address_ward",
    "address_district",
    "address_city",
    "details",
    "opening_hours",
    "location"
) VALUES
(
    '11111111-1111-4111-8111-111111111111',
    'Smatch Test Court Ba Dinh',
    'Seed court for API, ALB, and mobile smoke tests.',
    ARRAY['+84900000001'],
    '12 Kim Ma',
    'Kim Ma',
    'Ba Dinh',
    'Hanoi',
    '{"amenities":["parking","shower","racket_rental"],"surface":"wood","indoor":true}'::jsonb,
    '{"mon":"06:00-22:00","tue":"06:00-22:00","wed":"06:00-22:00","thu":"06:00-22:00","fri":"06:00-22:00","sat":"06:00-23:00","sun":"06:00-23:00"}'::jsonb,
    ST_SetSRID(ST_MakePoint(105.8218, 21.0321), 4326)
),
(
    '22222222-2222-4222-8222-222222222222',
    'Smatch Test Court Cau Giay',
    'Seed court with multiple sub-courts for booking smoke tests.',
    ARRAY['+84900000002'],
    '88 Tran Thai Tong',
    'Dich Vong Hau',
    'Cau Giay',
    'Hanoi',
    '{"amenities":["wifi","parking","drinks"],"surface":"synthetic","indoor":true}'::jsonb,
    '{"mon":"05:30-22:30","tue":"05:30-22:30","wed":"05:30-22:30","thu":"05:30-22:30","fri":"05:30-22:30","sat":"05:30-23:00","sun":"05:30-23:00"}'::jsonb,
    ST_SetSRID(ST_MakePoint(105.7908, 21.0366), 4326)
),
(
    '33333333-3333-4333-8333-333333333333',
    'Smatch Test Court Thanh Xuan',
    'Seed court near Thanh Xuan for nearby-search smoke tests.',
    ARRAY['+84900000003'],
    '21 Nguyen Trai',
    'Thuong Liet',
    'Thanh Xuan',
    'Hanoi',
    '{"amenities":["locker","parking"],"surface":"wood","indoor":false}'::jsonb,
    '{"mon":"06:00-21:30","tue":"06:00-21:30","wed":"06:00-21:30","thu":"06:00-21:30","fri":"06:00-21:30","sat":"06:00-22:00","sun":"06:00-22:00"}'::jsonb,
    ST_SetSRID(ST_MakePoint(105.8019, 20.9948), 4326)
)
ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "description" = EXCLUDED."description",
    "phone_numbers" = EXCLUDED."phone_numbers",
    "address_street" = EXCLUDED."address_street",
    "address_ward" = EXCLUDED."address_ward",
    "address_district" = EXCLUDED."address_district",
    "address_city" = EXCLUDED."address_city",
    "details" = EXCLUDED."details",
    "opening_hours" = EXCLUDED."opening_hours",
    "location" = EXCLUDED."location",
    "updated_at" = NOW();

INSERT INTO "sub_courts" ("id", "court_id", "name", "description", "is_active") VALUES
    ('11111111-1111-4111-8111-aaaaaaaaaaa1', '11111111-1111-4111-8111-111111111111', 'Court A', 'Main indoor court', true),
    ('11111111-1111-4111-8111-aaaaaaaaaaa2', '11111111-1111-4111-8111-111111111111', 'Court B', 'Secondary indoor court', true),
    ('22222222-2222-4222-8222-bbbbbbbbbbb1', '22222222-2222-4222-8222-222222222222', 'Court 1', 'Synthetic court 1', true),
    ('22222222-2222-4222-8222-bbbbbbbbbbb2', '22222222-2222-4222-8222-222222222222', 'Court 2', 'Synthetic court 2', true),
    ('22222222-2222-4222-8222-bbbbbbbbbbb3', '22222222-2222-4222-8222-222222222222', 'Court 3', 'Synthetic court 3', true),
    ('33333333-3333-4333-8333-ccccccccccc1', '33333333-3333-4333-8333-333333333333', 'Outdoor A', 'Outdoor court for smoke tests', true)
ON CONFLICT ("id") DO UPDATE SET
    "court_id" = EXCLUDED."court_id",
    "name" = EXCLUDED."name",
    "description" = EXCLUDED."description",
    "is_active" = EXCLUDED."is_active",
    "updated_at" = NOW();

INSERT INTO "pricing_rules" ("id", "court_id", "name", "day_type", "start_time", "end_time", "price_per_hour", "is_active") VALUES
    ('aaaaaaaa-0001-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'Weekday daytime', 'weekday', '06:00', '17:00', 90000, true),
    ('aaaaaaaa-0001-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'Weekday evening', 'weekday', '17:00', '22:00', 140000, true),
    ('bbbbbbbb-0002-4000-8000-000000000001', '22222222-2222-4222-8222-222222222222', 'Weekend standard', 'weekend', '06:00', '23:00', 160000, true),
    ('cccccccc-0003-4000-8000-000000000001', '33333333-3333-4333-8333-333333333333', 'Weekday standard', 'weekday', '06:00', '21:30', 80000, true)
ON CONFLICT ("id") DO UPDATE SET
    "court_id" = EXCLUDED."court_id",
    "name" = EXCLUDED."name",
    "day_type" = EXCLUDED."day_type",
    "start_time" = EXCLUDED."start_time",
    "end_time" = EXCLUDED."end_time",
    "price_per_hour" = EXCLUDED."price_per_hour",
    "is_active" = EXCLUDED."is_active",
    "updated_at" = NOW();
