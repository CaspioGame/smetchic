-- A zero resource represents a value the supplier did not publish.
ALTER TABLE global_tools DROP CONSTRAINT global_tools_resource_minutes_check;
ALTER TABLE global_tools ADD CONSTRAINT global_tools_resource_nonnegative
    CHECK (resource_minutes >= 0 AND resource_minutes < 'Infinity'::double precision);
ALTER TABLE global_tools ADD COLUMN source_url TEXT NOT NULL DEFAULT '';

INSERT INTO equipment_models
    (id, name, manufacturer, type, work_area_x_mm, work_area_y_mm, work_area_z_mm,
     max_rpm, spindle_power_kw, max_feed_mm_min, description)
VALUES
    ('stankom-dekart-s4040', 'DeKart S4040 (ШВП)', 'Станком', 'milling', 400, 400, 200, 24000, 1.5, 12000,
     'Настольный универсальный станок, жидкостное охлаждение, ER16. Цена от 299 000 ₽. https://stanok-chpu.ru/catalog/stanki_v_nalichii/frezerno-gravirovalnyy-stanok-s-chpu-universalnyy-dekart-s4040-nalichie/'),
    ('stankom-dekart-6090', 'DeKart 6090', 'Станком', 'milling', 600, 900, 220, 24000, 1.5, 12000,
     'Исполнение с ШВП. Цена от 250 000 ₽. https://stanok-chpu.ru/catalog/stanki_v_nalichii/frezerno_gravirovalnyiy_stanok_s_chpu_____derevo_obrabotka_dekart_6090_150_nalichie/'),
    ('stankom-dekart-6012', 'DeKart 6012', 'Станком', 'milling', 600, 1200, 220, 24000, 1.5, 12000,
     'Исполнение с ШВП. Цена от 299 000 ₽. https://stanok-chpu.ru/catalog/stanki_v_nalichii/frezerno-gravirovalnyy-stanok-s-chpu-stankom-dekart-6012_nalichie/'),
    ('stankom-dekart-1212', 'DeKart 1212', 'Станком', 'milling', 1200, 1200, 220, 24000, 1.5, 12000,
     'ШВП, ручная смена инструмента, жидкостное охлаждение, ER16. Цена 353 000 ₽. https://stanok-chpu.ru/catalog/stanki_v_nalichii/frezerno_gravirovalnyiy_stanok_s_chpu_derevo_obrabotka_dekart_1212_485_nalichie_kryglyi/'),
    ('stankom-dekart-atc-1631l', 'DeKart ATC 1631L', 'Станком', 'milling', 1600, 3100, 300, 18000, 4.5, 28000,
     'Исполнение с автоматической сменой инструмента и косозубой рейкой. Цена от 966 000 ₽. https://stanok-chpu.ru/catalog/stanki_v_nalichii/frezerno_gravirovalnyiy_stanok_s_chpu_dekart_atc__1631l_101_nalichie/')
ON CONFLICT DO NOTHING;

INSERT INTO global_tools (id, name, unit_price, resource_minutes, currency, source_url)
VALUES
    ('stankom-cl12-4-60-120', 'Алмазная чистовая фреза CL12-4-60(120)', 1710, 0, 'RUB', 'https://stanok-chpu.ru/catalog/komplektuyuschie/frezy/'),
    ('stankom-cl12-4-80-150', 'Алмазная чистовая фреза CL12-4-80(150)', 1805, 0, 'RUB', 'https://stanok-chpu.ru/catalog/komplektuyuschie/frezy/'),
    ('stankom-fp12-13-25-130', 'Алмазная черновая фреза FP12-13-25(130)', 1710, 0, 'RUB', 'https://stanok-chpu.ru/catalog/komplektuyuschie/frezy/')
ON CONFLICT (id) DO NOTHING;
