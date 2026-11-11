# Changelog

- Add PUT /devices/{id}/status endpoint to set a device's status to up or down without re-creating it. (#1)
- Added optional `site` and `role` query-parameter filters to `GET /devices` (exact, case-sensitive match; results still sorted by ID). (#2)
