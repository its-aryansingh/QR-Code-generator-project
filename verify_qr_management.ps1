$ErrorActionPreference = "Stop"

$baseUrl = "http://localhost:8082/api/v1"
$email = "test_mgmt_" + (Get-Random) + "@example.com"
$password = "password123"

function Test-Request {
    param (
        [string]$Method,
        [string]$Uri,
        [hashtable]$Body = @{},
        [hashtable]$Headers = @{}
    )
    try {
        $params = @{
            Method      = $Method
            Uri         = $Uri
            Headers     = $Headers
            ContentType = "application/json"
        }
        if ($Body.Count -gt 0) {
            $params.Body = ($Body | ConvertTo-Json -Depth 10)
        }
        $response = Invoke-RestMethod @params
        return $response
    }
    catch {
        Write-Host "Error calling $Uri" -ForegroundColor Red
        Write-Host $_.Exception.Message -ForegroundColor Red
        if ($_.Exception.Response) {
            $stream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            $responseBody = $reader.ReadToEnd()
            Write-Host "Response Body: $responseBody" -ForegroundColor Yellow
        }
        throw $_
    }
}

Write-Host "1. Registering user..." -ForegroundColor Cyan
$regRes = Test-Request -Method POST -Uri "$baseUrl/auth/register" -Body @{
    email      = $email
    password   = $password
    first_name = "Test"
    last_name  = "User"
}
Write-Host "User registered." -ForegroundColor Green

Write-Host "1.5. Logging in..." -ForegroundColor Cyan
$loginRes = Test-Request -Method POST -Uri "$baseUrl/auth/login" -Body @{
    email    = $email
    password = $password
}
$token = $loginRes.data.access_token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "Logged in. Token acquired." -ForegroundColor Green

Write-Host "`n2. Generating QR code..." -ForegroundColor Cyan
$genRes = Test-Request -Method POST -Uri "$baseUrl/qr/generate" -Headers $headers -Body @{
    content = "https://example.com"
    qr_type = "url"
    size    = 256
}
$qrID = $genRes.data.id
Write-Host "QR Generated. ID: $qrID" -ForegroundColor Green

Write-Host "`n3. Checking History..." -ForegroundColor Cyan
$histRes = Test-Request -Method GET -Uri "$baseUrl/qr/history" -Headers $headers
$count = $histRes.data.total
Write-Host "Total QRs: $count" -ForegroundColor Yellow
if ($count -lt 1) { throw "History count is 0" }
$found = $false
foreach ($qr in $histRes.data.records) {
    if ($qr.id -eq $qrID) { $found = $true; break }
}
if (-not $found) { throw "Generated QR not found in history" }
Write-Host "QR found in history." -ForegroundColor Green

Write-Host "`n3.5. Updating QR Title..." -ForegroundColor Cyan
try {
    $updateRes = Test-Request -Method PUT -Uri "$baseUrl/qr/$qrID" -Headers $headers -Body @{
        title = "Updated Title"
    }
    if ($updateRes.data.title -ne "Updated Title") {
        throw "Title update failed. Expected 'Updated Title', got '$($updateRes.data.title)'"
    }
    Write-Host "QR Title updated successfully." -ForegroundColor Green
}
catch {
    Write-Host "Update failed: $($_.Exception.Message)" -ForegroundColor Red
    throw $_
}


Write-Host "`n3.6. Updating QR Customization..." -ForegroundColor Cyan
$customization = @{
    foreground_color = "#FF0000"
    background_color = "#FFFFFF"
    corner_style     = "dots"
}
try {
    $updateRes = Test-Request -Method PUT -Uri "$baseUrl/qr/$qrID" -Headers $headers -Body @{
        customization = $customization
    }
    
    # Verify customization in response or fetch again
    if ($updateRes.data.customization.foreground_color -ne "#FF0000") {
        throw "Customization update failed. Expected #FF0000, got $($updateRes.data.customization.foreground_color)"
    }
    Write-Host "QR Customization updated successfully." -ForegroundColor Green
}
catch {
    Write-Host "Customization update failed: $($_.Exception.Message)" -ForegroundColor Red
    throw $_
}

Write-Host "`n4. Deleting QR code..." -ForegroundColor Cyan
try {
    Invoke-RestMethod -Method DELETE -Uri "$baseUrl/qr/$qrID" -Headers $headers
    Write-Host "Delete request successful." -ForegroundColor Green
}
catch {
    Write-Host "Delete failed: $($_.Exception.Message)" -ForegroundColor Red
    throw $_
}

Write-Host "`n5. Verifying Deletion..." -ForegroundColor Cyan
$histResAfter = Test-Request -Method GET -Uri "$baseUrl/qr/history" -Headers $headers
$foundAfter = $false
if ($histResAfter.data.records) {
    foreach ($qr in $histResAfter.data.records) {
        if ($qr.id -eq $qrID) { $foundAfter = $true; break }
    }
}
if ($foundAfter) { throw "QR still exists in history after deletion!" }
Write-Host "QR successfully removed from history." -ForegroundColor Green

Write-Host "`nSuccess! QR Management API verified." -ForegroundColor Green
