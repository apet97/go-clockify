# Finding: files (generic upload utility)

## Endpoint(s) probed
| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /file/image | 200 | live-probe 2026-08-04/05 |

## Live promotion (2026-08-04/05, clockify-ts-sdk Slice 1)

`POST /file/image` (`uploadImage`) is not workspace-scoped and does not
attach the upload to any entity by itself — it's a generic pre-step other
writes (project color/icon, user avatar, workspace logo) can point at
afterward. Uploaded a 1x1 PNG via multipart (`file` part); the response is
`{name, url}` matching `ImageUploadResponse`. There is no delete API for the
uploaded blob — this is an accepted, unavoidable footprint of proving this
route live (the file is not referenced by any workspace entity, so it has
no functional effect beyond storage).

| Method | Host | Path | Status | Fixture |
|---|---|---|---|---|
| POST | api.clockify.me | /file/image | 200 | live-probe 2026-08-04/05, https://avatar.cake.com/2026-08-05-00-23-33.e41c07c6bd5feb1be43cec378019046fbdc9400a0774019d15ae0a491d33759c.cake-avatar.png |
