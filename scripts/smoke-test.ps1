param(
  [string]$ApiBase = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"

function Assert-Ok($cond, $msg) {
  if (-not $cond) { throw $msg }
}

Write-Host "== GALA Smoke Test =="

# 1) Health
Write-Host "1) Health..."
$health = Invoke-RestMethod "$ApiBase/health" -Method GET
Assert-Ok ($health.status -eq "ok") "Health failed"
Write-Host "   OK"

# 2) Create job
Write-Host "2) Create job..."
$body = @{
  name="hello-e2e"
  params=@{ text="GALA E2E" }
} | ConvertTo-Json -Depth 5

$created = Invoke-RestMethod "$ApiBase/jobs" -Method POST -ContentType "application/json" -Body $body
$jobId = $created.job.id
Assert-Ok ($jobId -ne $null -and $jobId.Length -gt 0) "Job id missing"
Write-Host "   jobId=$jobId"

# 3) Poll job until DONE/FAILED
Write-Host "3) Wait job DONE..."
$max = 60
$job = $null
for ($i=0; $i -lt $max; $i++) {
  Start-Sleep -Seconds 1
  $job = Invoke-RestMethod "$ApiBase/jobs/$jobId" -Method GET
  $status = $job.job.status
  Write-Host "   status=$status"
  if ($status -eq "DONE" -or $status -eq "FAILED") { break }
}
Assert-Ok ($job.job.status -eq "DONE") "Job did not finish DONE"

# 4) Extract asset IDs
$outputs = $job.job.outputs
Assert-Ok ($outputs.Count -ge 1) "No outputs recorded"

$videoAssetId = $outputs[0].video_asset_id
$thumbAssetId = $outputs[0].thumbnail_asset_id

Assert-Ok ($videoAssetId) "video_asset_id missing"
Assert-Ok ($thumbAssetId) "thumbnail_asset_id missing"

Write-Host "4) Download assets..."
Invoke-WebRequest "$ApiBase/assets/$videoAssetId/content" -OutFile "hello.mp4" | Out-Null
Invoke-WebRequest "$ApiBase/assets/$thumbAssetId/content" -OutFile "hello.jpg" | Out-Null

$videoSize = (Get-Item "hello.mp4").Length
$thumbSize = (Get-Item "hello.jpg").Length

Assert-Ok ($videoSize -gt 10000) "Video too small (size=$videoSize)"
Assert-Ok ($thumbSize -gt 1000) "Thumb too small (size=$thumbSize)"

Write-Host "✅ Smoke test OK"
Write-Host "   hello.mp4 ($videoSize bytes)"
Write-Host "   hello.jpg ($thumbSize bytes)"
