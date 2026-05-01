$ErrorActionPreference = "Stop"
$BaseUrl = "http://localhost:8082/api/v1"

function Test-Endpoint {
    param($Name, $Method, $Path, $Body, $Token, $ExpectedStatus = 200)
    Write-Host "Testing $Name..." -NoNewline
    
    $Headers = @{}
    if ($Token) { $Headers["Authorization"] = "Bearer $Token" }
    
    try {
        $params = @{
            Uri         = "$BaseUrl$Path"
            Method      = $Method
            Headers     = $Headers
            ContentType = "application/json"
        }
        if ($Body) { $params.Body = ($Body | ConvertTo-Json -Depth 10) }
        
        $response = Invoke-RestMethod @params
        Write-Host " OK" -ForegroundColor Green
        return $response
    }
    catch {
        $response = $_.Exception.Response
        if ($response.StatusCode -eq $ExpectedStatus) {
            Write-Host " OK (Expected $($response.StatusCode))" -ForegroundColor Green
            return $response
        }
        Write-Host " FAILED" -ForegroundColor Red
        Write-Host "Status: $($response.StatusCode)"
        if ($response) {
            $stream = $response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            $body = $reader.ReadToEnd()
            Write-Host "Body: $body"
        }
        exit 1
    }
}

# 1. Register
$email = "settings_test_$(Get-Random)@example.com"
$password = "password123"
Test-Endpoint -Name "Register" -Method "POST" -Path "/auth/register" -Body @{ email = $email; password = $password }

# 2. Login
$loginRes = Test-Endpoint -Name "Login" -Method "POST" -Path "/auth/login" -Body @{ email = $email; password = $password }
$token = $loginRes.data.access_token

# 3. Get Profile
$profile = Test-Endpoint -Name "Get Profile" -Method "GET" -Path "/user/profile" -Token $token
if ($profile.data.email -ne $email) { Write-Error "Email mismatch" }

# 4. Update Name
Test-Endpoint -Name "Update Name" -Method "PUT" -Path "/user/profile" -Token $token -Body @{ name = "Updated Name" }

# 5. Verify Name Update
$profile = Test-Endpoint -Name "Verify Name" -Method "GET" -Path "/user/profile" -Token $token
if ($profile.data.name -ne "Updated Name") { Write-Error "Name not updated" }

# 6. Update Password
$newPassword = "newpassword456"
Test-Endpoint -Name "Update Password" -Method "PUT" -Path "/user/profile" -Token $token -Body @{ password = $newPassword }

# 7. Verify Login with New Password
$newLoginRes = Test-Endpoint -Name "Login New Password" -Method "POST" -Path "/auth/login" -Body @{ email = $email; password = $newPassword }
Write-Host "Token received for new password login"

Write-Host "`nAll Settings tests passed!" -ForegroundColor Green
