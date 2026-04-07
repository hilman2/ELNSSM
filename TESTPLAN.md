# ELNSSM Test Plan

## Overview

Comprehensive test plan covering unit tests, integration tests, and manual E2E verification.
Run all tests: `go test ./... -v -race`

---

## 1. Unit Tests

### 1.1 internal/model/ - Domain Types
| Test | Description |
|------|-------------|
| TestServiceStateConstants | Verify all state string values match expected |
| TestStartupTypeConstants | Verify auto/manual/disabled/delayed-auto |
| TestStopSignalConstants | Verify ctrl_c/ctrl_break/wm_close/terminate |
| TestRestartModeConstants | Verify always/on_failure/never |
| TestHealthCheckTypeConstants | Verify http/tcp/script |

### 1.2 internal/config/ - Configuration
| Test | Description |
|------|-------------|
| TestDefaultConfig | Default values are sensible (listen addr, timeouts, etc.) |
| TestDefaultDataDir | Falls back to C:\ProgramData when env var missing |
| TestLoadConfig | Parse valid YAML into Config struct |
| TestLoadConfig_FileNotFound | Returns error for missing file |
| TestLoadConfig_InvalidYAML | Returns error for corrupted YAML |
| TestSaveConfig | Round-trip: save then load, verify equality |
| TestSaveConfig_CreatesDir | Creates parent directories if missing |
| TestConfigPaths | ConfigDir, ServicesDir, DataPath, LogsDir, ServiceLogDir |
| TestLoadServiceConfig | Parse service YAML with all fields |
| TestLoadServiceConfig_Durations | Parse "30s", "5m", "1h", "168h" correctly |
| TestLoadServiceConfig_InvalidDuration | Error on "abc", "30x", negative |
| TestLoadServiceConfig_HealthChecks | Parse multiple health check types |
| TestLoadServiceConfig_Notifications | Parse notification overrides |
| TestSaveServiceConfig_RoundTrip | Save → load → compare |
| TestLoadAllServiceConfigs | Load directory with multiple YAML files |
| TestLoadAllServiceConfigs_EmptyDir | Returns empty slice, no error |
| TestLoadAllServiceConfigs_SkipDirs | Ignores subdirectories |
| TestLoadAllServiceConfigs_SkipNonYAML | Ignores .txt, .json etc. |
| TestServiceToYAML | Convert model.Service to YAML struct |
| TestServiceToYAML_ScheduledRestart | Cron expression preserved |

### 1.3 internal/store/ - Persistence
| Test | Description |
|------|-------------|
| TestNewBoltStore | Creates DB file and all buckets |
| TestNewBoltStore_CreatesDir | Creates parent directory if missing |
| TestNewBoltStore_Locked | Returns error if DB locked by another process |
| TestSaveAndGetService | Save service, get by ID, verify all fields |
| TestGetService_NotFound | Returns error for non-existent ID |
| TestListServices_Empty | Returns empty slice on fresh DB |
| TestListServices_Multiple | Returns all saved services |
| TestDeleteService | Save → delete → get returns not found |
| TestDeleteService_NotFound | Silently succeeds (idempotent) |
| TestSaveAndGetServiceState | Round-trip runtime state |
| TestGetServiceState_NotFound | Returns zero-value state |
| TestAppendAndListEvents | Insert events, list newest-first |
| TestListEvents_FilterByServiceID | Only matching service events returned |
| TestListEvents_FilterByType | Only matching event type returned |
| TestListEvents_FilterBySince | Only events after timestamp |
| TestListEvents_Limit | Respects limit parameter |
| TestListEvents_Empty | Returns empty slice |
| TestAppendHealthResult | Store result in per-service sub-bucket |
| TestGetHealthHistory | Returns chronological order |
| TestGetHealthHistory_Limit | Respects limit parameter |
| TestGetHealthHistory_Empty | Returns empty slice for unknown service |
| TestHealthResultPruning | Keeps only 1000 entries per service |
| TestRunMigrations_Fresh | Sets schema version to "1" |
| TestRunMigrations_SameVersion | No-op on current version |
| TestRunMigrations_UnknownVersion | Returns error |
| TestClose | Closes DB without error |

### 1.4 internal/health/ - Health Checks
| Test | Description |
|------|-------------|
| **HTTPChecker** | |
| TestHTTPChecker_Healthy | Server returns expected 200 → healthy |
| TestHTTPChecker_UnhealthyStatus | Server returns 500 → unhealthy |
| TestHTTPChecker_CustomExpectStatus | ExpectStatus=204, server returns 204 → healthy |
| TestHTTPChecker_BodyMatch | Response body contains expected string → healthy |
| TestHTTPChecker_BodyNoMatch | Response body missing expected string → unhealthy |
| TestHTTPChecker_Timeout | Server hangs → unhealthy with timeout error |
| TestHTTPChecker_ConnectionRefused | No server → unhealthy |
| TestHTTPChecker_DefaultMethod | GET used when Method is empty |
| TestHTTPChecker_CustomMethod | POST used when configured |
| TestHTTPChecker_Duration | Duration > 0 after check |
| TestHTTPChecker_LargeBody | 1MB body limit respected |
| **TCPChecker** | |
| TestTCPChecker_Healthy | Open port → healthy |
| TestTCPChecker_Unhealthy | Closed port → unhealthy |
| TestTCPChecker_Timeout | Firewall drops → timeout → unhealthy |
| TestTCPChecker_DefaultTimeout | 5s used when Timeout=0 |
| TestTCPChecker_ContextCancel | Cancelled context → returns immediately |
| **ScriptChecker** | |
| TestScriptChecker_Healthy | Exit code 0 → healthy |
| TestScriptChecker_Unhealthy | Exit code 1 → unhealthy |
| TestScriptChecker_Timeout | Long script → timeout → unhealthy |
| TestScriptChecker_OutputTruncation | Long output truncated to 500/200 chars |
| TestScriptChecker_NonExistent | Script not found → unhealthy |
| **NewChecker Factory** | |
| TestNewChecker_HTTP | Returns HTTPChecker |
| TestNewChecker_TCP | Returns TCPChecker |
| TestNewChecker_Script | Returns ScriptChecker |
| TestNewChecker_Unknown | Returns error |
| **Runner** | |
| TestRunner_RunsChecks | Executes checks at configured interval |
| TestRunner_StartDelay | Waits before first check |
| TestRunner_ConsecutiveFailures | Reports failure only after N retries |
| TestRunner_Recovery | Resets failure count on success |
| TestRunner_FailureChannel | Sends result when threshold exceeded |
| TestRunner_FailureChannelFull | Non-blocking send, doesn't hang |
| TestRunner_Stop | Cancels all check goroutines |
| TestRunner_StoresResults | Calls store.AppendHealthResult |
| TestRunner_StoreError | Continues running if store fails |
| **RingBuffer** | |
| TestRingBuffer_Empty | GetAll returns empty, Latest returns false |
| TestRingBuffer_Add | Add items, GetAll returns them in order |
| TestRingBuffer_Wraparound | After capacity, oldest replaced |
| TestRingBuffer_Latest | Returns most recent entry |
| TestRingBuffer_SizeOne | Works correctly with size=1 |
| TestRingBuffer_Concurrent | No race with concurrent Add/GetAll |

### 1.5 internal/notify/ - Notifications
| Test | Description |
|------|-------------|
| **Dispatcher** | |
| TestDispatcher_NoNotifiers | Dispatch does nothing, no panic |
| TestDispatcher_SendsToAll | Dispatch calls Send on all notifiers |
| TestDispatcher_Cooldown | Second dispatch within window is suppressed |
| TestDispatcher_CooldownByServiceAndType | Different service or type bypasses cooldown |
| TestDispatcher_CooldownExpired | Dispatch after cooldown succeeds |
| TestDispatcher_NotifierError | Logs error, doesn't propagate |
| TestDispatcher_ParseCooldown | Custom duration parsed from config |
| **EmailNotifier** | |
| TestEmailNotifier_Send | Constructs and sends email via SMTP |
| TestEmailNotifier_NoRecipients | Returns nil immediately |
| TestEmailNotifier_SubjectFormat | "[ELNSSM] event.type: serviceID" |
| TestEmailNotifier_TLS | Uses TLS dialer when configured |
| TestEmailNotifier_NoAuth | Skips auth when username empty |
| **WebhookNotifier** | |
| TestWebhook_Send | POST with correct body/headers |
| TestWebhook_EventFilter | Skips events not in Events list |
| TestWebhook_AllEvents | Empty Events list sends all |
| TestWebhook_BodyTemplate | Template variables rendered correctly |
| TestWebhook_DefaultBody | JSON format when no template |
| TestWebhook_DefaultMethod | POST when Method empty |
| TestWebhook_ErrorStatus | Returns error for 400+ responses |
| TestWebhook_Timeout | 10s client timeout |
| TestWebhook_CustomHeaders | Headers set on request |

### 1.6 internal/logging/ - Log Capture & Streaming
| Test | Description |
|------|-------------|
| **Capture** | |
| TestCapture_StdoutStderr | Captures both streams to separate files |
| TestCapture_CombinedOutput | Both streams to same file |
| TestCapture_DefaultFileNames | "stdout.log" and "stderr.log" |
| TestCapture_LogRotation | Lumberjack settings applied |
| TestCapture_GetLogPath | Returns correct paths |
| TestCapture_StreamerBroadcast | Lines sent to streamer |
| TestCapture_NilStreamer | No panic when streamer is nil |
| TestCapture_Close | Closes all writers |
| TestCapture_CreatesDir | Creates log directory |
| **Streamer** | |
| TestStreamer_RegisterAndBroadcast | Client receives matching messages |
| TestStreamer_StreamFilter | stdout client doesn't get stderr |
| TestStreamer_CombinedStream | combined client gets both |
| TestStreamer_ServiceFilter | Client only gets its service's logs |
| TestStreamer_SlowClient | Dropped messages, no hang |
| TestStreamer_Unregister | Client removed, channel closed |
| TestStreamer_MultipleClients | All matching clients receive |
| **RingBuffer** | |
| TestRingBuffer_Empty | GetAll → empty, Latest → false |
| TestRingBuffer_FillAndWrap | Oldest entries replaced correctly |
| TestRingBuffer_ChronologicalOrder | GetAll always chronological |

### 1.7 internal/process/ - Process Management (Windows-specific)
| Test | Description |
|------|-------------|
| **Wrapper** | |
| TestWrapper_StartStop | Start process, verify PID, stop gracefully |
| TestWrapper_StartAlreadyRunning | Returns error |
| TestWrapper_StopNotRunning | Returns nil (no-op) |
| TestWrapper_CrashDetection | Process exits non-zero → Crashed=true |
| TestWrapper_CleanExit | Process exits 0 → Crashed=false |
| TestWrapper_EnvironmentVars | Child process inherits + overlays env |
| TestWrapper_WorkingDir | Child runs in specified directory |
| TestWrapper_StdoutCapture | Stdout readable |
| TestWrapper_StderrCapture | Stderr readable |
| TestWrapper_ForceKill | Timeout → Job Object terminate |
| TestWrapper_CtrlC | CTRL_C sent on stop (default signal) |
| TestWrapper_CtrlBreak | CTRL_BREAK sent when configured |
| TestWrapper_Terminate | Skip graceful, direct kill |
| TestWrapper_PIDTracking | PID() returns correct value |
| TestWrapper_IsRunning | True while running, false after exit |
| TestWrapper_Close | Releases Job Object and cancel func |
| **JobObject** | |
| TestJobObject_CreateAndClose | Create, verify handle, close |
| TestJobObject_Assign | Assign process handle |
| TestJobObject_Terminate | Kill all processes in job |
| TestJobObject_DoubleClose | Second close is no-op |
| **Signals** | |
| TestSendCtrlC | Sends CTRL_C_EVENT to process group |
| TestSendCtrlBreak | Sends CTRL_BREAK_EVENT |
| TestSendWMClose | Finds window and sends WM_CLOSE |
| TestSendWMClose_NoWindows | Returns error for windowless process |

### 1.8 internal/manager/ - Service Orchestration
| Test | Description |
|------|-------------|
| TestManager_Add | Registers service, saves config+store |
| TestManager_AddDuplicate | Returns error for existing ID |
| TestManager_Remove | Removes stopped service |
| TestManager_RemoveRunning | Returns error |
| TestManager_RemoveNotFound | Returns error |
| TestManager_Start | Starts service, sets running state |
| TestManager_StartNotFound | Returns error |
| TestManager_StartAlreadyRunning | Returns error |
| TestManager_Stop | Stops running service |
| TestManager_StopNotRunning | No-op, returns nil |
| TestManager_Restart | Stop + Start sequence |
| TestManager_Get | Returns ManagedService by ID |
| TestManager_GetNotFound | Returns false |
| TestManager_List | Returns all services with uptime |
| TestManager_StopAll | Stops all running services in parallel |
| TestManager_LoadAll | Loads configs and restores state |
| TestManager_AutoStart | Starts auto and delayed-auto services |
| TestManager_Update | Updates config, preserves runtime state |
| TestManager_CrashRestart | Crash → restart with policy |
| TestManager_CrashMaxRetries | Exceed retries → failed state |
| TestManager_BackoffCalculation | Exponential delay with cap |
| TestManager_RestartNever | No restart on crash |
| TestManager_RestartAlways | Restart even on clean exit |
| TestManager_RestartOnFailure | Restart only on non-zero exit |
| TestManager_EmitEvent | Events stored and dispatched |
| TestManager_HealthCheckRestart | Health failure triggers restart |
| TestManager_MonitorStopSignal | stopCh stops monitor goroutine |

### 1.9 internal/api/ - REST API Handlers
| Test | Description |
|------|-------------|
| **Middleware** | |
| TestIPWhitelist_Allowed | Whitelisted IP passes through |
| TestIPWhitelist_Blocked | Non-whitelisted IP returns 403 |
| TestIPWhitelist_Empty | Empty whitelist allows all |
| TestIPWhitelist_IPv6 | ::1 handled correctly |
| TestExtractIP_XForwardedFor | First IP from XFF header |
| TestExtractIP_XRealIP | XRI header used |
| TestExtractIP_RemoteAddr | Fallback to RemoteAddr |
| TestCORS_Headers | Correct headers set |
| TestCORS_Options | OPTIONS returns 200 |
| **Service Handlers** | |
| TestAPI_ListServices | GET /services returns list |
| TestAPI_ListServices_Empty | Returns empty array, not null |
| TestAPI_GetService | GET /services/{id} returns detail |
| TestAPI_GetService_NotFound | Returns 404 |
| TestAPI_AddService | POST /services creates service |
| TestAPI_AddService_Defaults | Missing fields get defaults |
| TestAPI_AddService_MissingID | Returns 400 |
| TestAPI_AddService_MissingExe | Returns 400 |
| TestAPI_AddService_InvalidJSON | Returns 400 |
| TestAPI_AddService_Duplicate | Returns 409 |
| TestAPI_UpdateService | PUT /services/{id} updates config |
| TestAPI_DeleteService | DELETE /services/{id} removes |
| TestAPI_DeleteService_Running | Returns 409 |
| TestAPI_StartService | POST /services/{id}/start |
| TestAPI_StopService | POST /services/{id}/stop |
| TestAPI_RestartService | POST /services/{id}/restart |
| **Log Handlers** | |
| TestAPI_GetLogs | Returns last N lines |
| TestAPI_GetLogs_DefaultLines | 100 lines when not specified |
| TestAPI_GetLogs_NotFound | 404 for missing log |
| TestAPI_DownloadLog | Content-Disposition header set |
| TestAPI_StreamLogs | WebSocket upgrade succeeds |
| **Health Handlers** | |
| TestAPI_GetHealth | Returns latest health check |
| TestAPI_GetHealth_NotFound | 404 for unknown service |
| TestAPI_GetHealthHistory | Returns history with limit |
| **System Handlers** | |
| TestAPI_SystemStatus | Returns uptime, counts, version |
| TestAPI_Version | Returns version/commit/date |
| TestAPI_ListEvents | Returns filtered events |
| **Config Handlers** | |
| TestAPI_GetConfig | Returns sanitized config (no passwords) |
| TestAPI_UpdateConfig | Accepts and acknowledges |
| **Response Helpers** | |
| TestWriteJSON | Correct envelope format |
| TestWriteJSONList | Includes total count |
| TestWriteError | Error envelope with code+message |

---

## 2. Integration Tests

### 2.1 Store + Config Integration
| Test | Description |
|------|-------------|
| TestStoreConfigRoundTrip | Save service config YAML → load into store → compare |
| TestStoreStateRecovery | Save state → close DB → reopen → verify state |
| TestStoreEventPruning_UnderLoad | Insert 2000 events, verify query still fast |

### 2.2 Manager + Process Integration
| Test | Description |
|------|-------------|
| TestFullServiceLifecycle | Add → Start → verify PID → Stop → verify stopped |
| TestCrashAndRestart | Start process that exits(1) → verify restart occurs |
| TestCrashMaxRetries | Process crashes repeatedly → verify failed state |
| TestGracefulShutdown | Start service → StopAll → verify clean shutdown |
| TestConcurrentStartStop | Parallel start/stop calls → no deadlock |
| TestDelayedAutoStart | Service with delayed-auto starts after 30s |

### 2.3 Health + Manager Integration
| Test | Description |
|------|-------------|
| TestHealthCheckTriggersRestart | HTTP check fails → service restarted |
| TestHealthCheckRecovery | Unhealthy → healthy transition logged |
| TestHealthCheckStartDelay | No checks run during start_delay period |

### 2.4 API + Manager Integration
| Test | Description |
|------|-------------|
| TestAPIFullCRUD | Create → Read → Update → Delete via HTTP |
| TestAPIStartStop | Start/Stop via API, verify state changes |
| TestAPILogStreaming | Start service → connect WS → receive logs |
| TestAPIEventStream | Trigger events → verify in event list |

---

## 3. End-to-End Manual Tests

### 3.1 Installation & Setup
```
[ ] elnssm version                          → shows dev/version info
[ ] elnssm --help                           → shows all commands
[ ] elnssm serve                            → starts Guardian on :9100
[ ] http://localhost:9100                    → Web GUI loads
[ ] http://localhost:9100/api/v1/system/status → JSON response with version
```

### 3.2 Service Management via CLI
```
[ ] elnssm add ping-test "C:\Windows\System32\ping.exe" -t localhost
    → "Service added" message
[ ] elnssm list
    → Shows ping-test with state=stopped
[ ] elnssm start ping-test
    → "Service started" message
[ ] elnssm status ping-test
    → Shows state=running, PID, uptime
[ ] elnssm logs ping-test -n 10
    → Shows last 10 lines of ping output
[ ] elnssm logs ping-test --follow
    → Live streaming of ping output, Ctrl+C to stop
[ ] elnssm restart ping-test
    → "Service restarted", new PID
[ ] elnssm stop ping-test
    → "Service stopped"
[ ] elnssm remove ping-test
    → "Service removed"
```

### 3.3 Service Management via Web GUI
```
[ ] Open http://localhost:9100
[ ] Dashboard shows no services initially
[ ] Click "+ Add Service"
    → Fill: ID=web-test, Exe=ping.exe, Args=-t localhost
    → Click "Add Service"
[ ] Dashboard shows web-test with state=stopped
[ ] Click "Start" → state changes to running, PID shown
[ ] Click "Logs" → log modal shows live ping output
[ ] Click "Restart" → new PID assigned
[ ] Click "Stop" → state changes to stopped
[ ] Click "Remove" → confirm → service removed
[ ] Auto-refresh updates every 5s
```

### 3.4 Crash Recovery & Restart Policy
```
[ ] Add service running: cmd /C "exit 1"
    → Configure restart_policy: on_failure, delay=2s, max_retries=3
[ ] Start service → crashes immediately
[ ] Verify: restarts 3 times with increasing delay (2s, 4s, 8s)
[ ] After 3rd restart: state=failed
[ ] Verify: events logged (service.crashed, service.restarted, restart.limit_reached)
[ ] API: GET /api/v1/events → all events present
```

### 3.5 Restart Policy: Always
```
[ ] Add service running: cmd /C "echo hello && exit 0"
    → Configure restart_policy: always, delay=3s
[ ] Start → exits 0 → restarts after 3s → repeat
[ ] Stop → restarts stop
```

### 3.6 Health Checks
```
[ ] Start a simple HTTP server (Python: python -m http.server 8888)
[ ] Add service wrapping that server with health check:
    type: http, target: http://localhost:8888, interval: 10s, retries: 2
[ ] Start service → health check shows healthy
[ ] Kill the HTTP server process directly (not via ELNSSM)
[ ] Wait for 2 check intervals → health_check.failed event
[ ] If restart_on_health_fail=true → service restarts
[ ] Restart HTTP server → health_check.recovered event
```

### 3.7 TCP Health Check
```
[ ] Start service that listens on TCP port
[ ] Configure TCP health check on that port
[ ] Verify healthy while port open
[ ] Block/close port → unhealthy detected
```

### 3.8 Script Health Check
```
[ ] Create check.bat: curl -s http://localhost:8888/health | findstr "OK"
[ ] Configure script health check with target=check.bat
[ ] Verify healthy when endpoint returns "OK"
[ ] Change endpoint response → unhealthy
```

### 3.9 Notifications (Email)
```
[ ] Configure SMTP in elnssm.yaml (use mailtrap.io or local mailhog)
[ ] Trigger service crash
[ ] Verify email received with correct subject/body
[ ] Trigger another crash within cooldown → no email
[ ] Wait for cooldown → trigger again → email received
```

### 3.10 Notifications (Webhook)
```
[ ] Start webhook receiver (https://webhook.site or local server)
[ ] Configure webhook in elnssm.yaml
[ ] Trigger service crash
[ ] Verify POST received with correct JSON body
[ ] Verify custom headers applied
[ ] Verify body_template rendering
[ ] Verify event filtering (only subscribed events)
```

### 3.11 Log Management
```
[ ] Start service with verbose output
[ ] Verify stdout.log created in %ProgramData%\ELNSSM\logs\{service}\
[ ] Verify log rotation (configure small max_size for testing)
[ ] Verify compressed backups created (.gz)
[ ] API: GET /api/v1/services/{id}/logs?lines=50 → last 50 lines
[ ] API: GET /api/v1/services/{id}/logs/download → file download
[ ] WebSocket: connect to /logs/stream → live output
```

### 3.12 Environment Variables & Working Directory
```
[ ] Add service with env vars: TEST_VAR=hello
[ ] Add script that echoes %TEST_VAR%
[ ] Start → verify "hello" in logs
[ ] Set working_dir to specific path
[ ] Add script that echoes %CD%
[ ] Start → verify correct working directory in logs
```

### 3.13 Stop Signals
```
[ ] Test with ctrl_c signal → process receives CTRL_C
[ ] Test with ctrl_break signal → process receives CTRL_BREAK
[ ] Test with terminate signal → direct TerminateProcess
[ ] Test stop_timeout: set to 5s, start long-running process
    → Send stop → wait 5s → force killed
```

### 3.14 Process Priority
```
[ ] Add service with priority=high
[ ] Start → verify in Task Manager that priority is High
[ ] Add service with priority=below_normal
[ ] Start → verify Below Normal priority
```

### 3.15 Security: IP Whitelist
```
[ ] Default config: only 127.0.0.1 and ::1
[ ] Access from localhost → allowed
[ ] Access from different machine (or via curl with spoofed header) → 403 Forbidden
[ ] Add remote IP to whitelist → access allowed
[ ] Empty whitelist → all IPs allowed
```

### 3.16 Config Persistence
```
[ ] Add service via API
[ ] Verify YAML file created in %ProgramData%\ELNSSM\config\services\
[ ] Edit YAML manually → restart Guardian → changes reflected
[ ] Verify bbolt DB has runtime state after service runs
```

### 3.17 Guardian Recovery
```
[ ] Install Guardian as Windows service: elnssm install
[ ] Start Guardian: sc start ELNSSM
[ ] Add and start a managed service
[ ] Kill Guardian process directly (taskkill /F)
[ ] Verify Guardian auto-restarts (SCM recovery actions)
[ ] Verify managed service state is recovered
```

### 3.18 Concurrent Operations
```
[ ] Add 10 services
[ ] Start all 10 simultaneously
[ ] Verify all reach running state
[ ] StopAll → all stopped in parallel
[ ] No deadlocks, no panics
```

### 3.19 Edge Cases
```
[ ] Add service with very long arguments list
[ ] Add service with unicode in name/path
[ ] Add service with spaces in executable path
[ ] Start service where executable doesn't exist → proper error
[ ] Start service where working_dir doesn't exist → proper error
[ ] Remove service while Guardian is shutting down
[ ] Rapid start/stop/start sequence on same service
```

---

## 4. Performance Tests

| Test | Description |
|------|-------------|
| BenchmarkStoreAppendEvent | Throughput of event insertion |
| BenchmarkStoreListEvents | Query speed with 10K events |
| BenchmarkStreamerBroadcast | WebSocket fan-out to 100 clients |
| BenchmarkHealthCheckHTTP | HTTP health check throughput |
| Test10Services | 10 services running with health checks |
| Test50Services | 50 services running simultaneously |

---

## 5. Race Condition Tests

Run with: `go test ./... -race`

| Area | What to verify |
|------|----------------|
| Manager.services map | Concurrent Add/Remove/Get/List |
| ManagedService.mu | Concurrent Start/Stop on same service |
| Wrapper.mu | Concurrent PID/IsRunning/Start/Stop |
| Streamer.clients | Concurrent Register/Unregister/Broadcast |
| RingBuffer.mu | Concurrent Add/GetAll/Latest |
| Dispatcher.cooldowns | Concurrent Dispatch calls |
| BoltStore | Concurrent read/write transactions |

---

## 6. Test Infrastructure

### Test Helpers Needed
- **MockStore**: In-memory implementation of `store.Store` interface
- **MockNotifier**: Records dispatched events for assertion
- **TestHTTPServer**: httptest.Server for health check tests
- **TestTCPServer**: net.Listen for TCP health check tests
- **TestProcess**: Simple .exe that exits with configurable code / runs forever
- **TestWebSocket**: gorilla/websocket test client

### Test Executable
Create `testdata/test_service.go` - a simple program that:
- Prints lines to stdout/stderr at intervals
- Exits with configurable exit code (via env var `EXIT_CODE`)
- Listens on configurable HTTP port (via env var `HTTP_PORT`)
- Responds to health checks on /health
- Runs until CTRL_C or timeout

---

## 7. Test Execution Order

1. `go test ./internal/model/...` - Types (no deps)
2. `go test ./internal/config/...` - Config (needs tempdir)
3. `go test ./internal/store/...` - Store (needs tempdir)
4. `go test ./internal/health/...` - Health checks (needs mock HTTP/TCP)
5. `go test ./internal/notify/...` - Notifications (needs mock SMTP/HTTP)
6. `go test ./internal/logging/...` - Logging (needs tempdir + mock WS)
7. `go test ./internal/process/...` - Process (Windows-specific, needs test exe)
8. `go test ./internal/manager/...` - Manager (needs mocks for store/process)
9. `go test ./internal/api/...` - API (needs httptest + mocks)
10. `go test ./... -race` - Full race detection pass
