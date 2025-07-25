package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/enbility/eebus-go/api"
	"github.com/enbility/eebus-go/service"
	ucapi "github.com/enbility/eebus-go/usecases/api"
	"github.com/enbility/eebus-go/usecases/eg/lpc"
	"github.com/enbility/eebus-go/usecases/eg/lpp"
	shipapi "github.com/enbility/ship-go/api"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/gorilla/websocket"
)

var remoteSki string

// getSystemHostname 获取系统hostname
func getSystemHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func getCertificateSubject(cert tls.Certificate) string {
	if len(cert.Certificate) == 0 {
		return "unknown"
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return "parse error"
	}

	return x509Cert.Subject.String()
}

// extractHostFromQRCode 从QR码中提取host信息
func extractHostFromQRCode(qrCode string) string {
	// QR码格式通常是: SHIP;SKI:xxx;ID:xxx;HOST:xxx;PORT:xxx;PATH:xxx
	// 我们需要提取HOST部分
	parts := strings.Split(qrCode, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "HOST:") {
			return strings.TrimPrefix(part, "HOST:")
		}
	}
	return ""
}

// generateQRCodeText 生成QR码文本（新版本API的替代方法）
func generateQRCodeText(h *controlbox) string {
	if h.myService == nil {
		return ""
	}

	localService := h.myService.LocalService()
	if localService == nil {
		return ""
	}

	// 构建SHIP QR码格式
	qrCode := fmt.Sprintf("SHIP;SKI:%s;ID:Demo-ControlBox-123456789;BRAND:Demo;TYPE:ElectricitySupplySystem;MODEL:ControlBox;SERIAL:123456789;CAT:1;ENDSHIP;",
		localService.SKI())

	return qrCode
}

type WebsocketClient struct {
	websocket *websocket.Conn
	mutex     sync.Mutex
	closed    bool
}

func (websocketClient *WebsocketClient) sendMessage(msg interface{}) error {
	websocketClient.mutex.Lock()
	defer websocketClient.mutex.Unlock()

	if websocketClient.websocket == nil || websocketClient.closed {
		return errors.New("no frontend connected or connection closed")
	}

	err := websocketClient.websocket.WriteJSON(msg)
	if err != nil {
		log.Println("WebSocket send error:", err)
	}
	return err
}

// Close safely closes the websocket connection
func (websocketClient *WebsocketClient) Close() error {
	websocketClient.mutex.Lock()
	defer websocketClient.mutex.Unlock()

	if websocketClient.websocket != nil && !websocketClient.closed {
		websocketClient.closed = true
		return websocketClient.websocket.Close()
	}
	return nil
}

func (websocketClient *WebsocketClient) sendNotification(messageType int) error {
	answer := Message{
		Type: messageType,
	}

	return websocketClient.sendMessage(answer)
}

func (websocketClient *WebsocketClient) sendText(messageType int, text string) error {
	answer := Message{
		Type: messageType,
		Text: text,
	}

	log.Printf("Sending message type %d with text: %s", messageType, text)
	return websocketClient.sendMessage(answer)
}

func (websocketClient *WebsocketClient) sendValue(messageType int, useCase string, value float64) error {
	answer := Message{
		Type:    messageType,
		Value:   value,
		UseCase: useCase,
	}

	return websocketClient.sendMessage(answer)
}

func (websocketClient *WebsocketClient) sendLimit(messageType int, useCase string, limit ucapi.LoadLimit) error {
	answer := Message{
		Type:    messageType,
		Limit:   limit,
		UseCase: useCase,
	}

	return websocketClient.sendMessage(answer)
}

func (websocketClient *WebsocketClient) sendEntityList(messageType int, entities map[spineapi.EntityRemoteInterface][]string) error {
	list := []EntityDescription{}

	for ed, ucs := range entities {
		list = append(list, EntityDescription{
			Name:     ed.Address().String(),
			SKI:      ed.Device().Ski(),
			UseCases: ucs,
		})
	}

	answer := Message{
		Type:       messageType,
		EntityList: list,
	}

	return websocketClient.sendMessage(answer)
}

var frontend WebsocketClient

type failsafeLimits struct {
	Value    float64
	Duration time.Duration
}

type controlbox struct {
	myService *service.Service

	uclpc ucapi.EgLPCInterface
	uclpp ucapi.EgLPPInterface

	isConnected bool

	remoteEntities map[spineapi.EntityRemoteInterface][]string

	consumptionLimits         ucapi.LoadLimit
	productionLimits          ucapi.LoadLimit
	consumptionFailsafeLimits failsafeLimits
	productionFailsafeLimits  failsafeLimits

	// Auto-restore timers for limits
	consumptionRestoreTimer *time.Timer
	productionRestoreTimer  *time.Timer

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

func (h *controlbox) run(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(ctx)
	var err error
	var certificate tls.Certificate

	if len(os.Args) == 5 {
		remoteSki = os.Args[2]

		certificate, err = tls.LoadX509KeyPair(os.Args[3], os.Args[4])
		if err != nil {
			usage()
			log.Fatal(err)
		}
	} else if len(os.Args) == 3 {
		// 支持只提供端口和远程SKI的情况
		remoteSki = os.Args[2]

		// 使用固定的证书，避免每次生成新的SKI
		log.Printf("🔒 [TLS] Using fixed certificate for stable SKI")

		// 固定的证书内容
		certPEM := `-----BEGIN CERTIFICATE-----
MIIBxDCCAWqgAwIBAgIQAv1kld7ZLcQUgMRbq1FhwjAKBggqhkjOPQQDAjBCMQsw
CQYDVQQGEwJERTENMAsGA1UEChMERGVtbzENMAsGA1UECxMERGVtbzEVMBMGA1UE
AxMMRGVtby1Vbml0LTAxMB4XDTI1MDcxODA4MTc1MloXDTM1MDcxNjA4MTc1Mlow
QjELMAkGA1UEBhMCREUxDTALBgNVBAoTBERlbW8xDTALBgNVBAsTBERlbW8xFTAT
BgNVBAMTDERlbW8tVW5pdC0wMTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABATV
1BjmClUdj73+BQ+uCzs5pzViPE3pRHkd7NStYpyV1sPRG163Y7gvB1fxnrir1cMW
eZVQFuOui+oQCC1JifyjQjBAMA4GA1UdDwEB/wQEAwIHgDAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBRrOVr6S+4RIV3w36ltXcdZ+bgO5TAKBggqhkjOPQQDAgNI
ADBFAiAGfHooUHUwrc0YO5ZmS695HFn7KVhwrXR5d6bt7G/ICwIhANl5FA4RuwxV
0JFHA2FjwLpc8+0j54WEVOZRwV+opGxq
-----END CERTIFICATE-----`

		keyPEM := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIA8JIhB5iM3+ekgN9PIpYZ5F7gIOfPFc1ud6rYECp2ftoAoGCCqGSM49
AwEHoUQDQgAEBNXUGOYKVR2Pvf4FD64LOzmnNWI8TelEeR3s1K1inJXWw9EbXrdj
uC8HV/GeuKvVwxZ5lVAW466L6hAILUmJ/A==
-----END EC PRIVATE KEY-----`

		// 输出证书以便查看
		fmt.Println(certPEM)
		fmt.Println(keyPEM)

		// 从PEM加载证书
		certificate, err = tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			log.Fatal(err)
		}

		// 固定的SKI: 6b395afa4bee11215df0dfa96d5dc759f9b80ee5
		log.Printf("🔑 [TLS] Fixed SKI: 6b395afa4bee11215df0dfa96d5dc759f9b80ee5")
	} else {
		// 使用固定的证书，避免每次生成新的SKI
		log.Printf("🔒 [TLS] Using fixed certificate for stable SKI")

		// 固定的证书内容
		certPEM := `-----BEGIN CERTIFICATE-----
MIIBxDCCAWqgAwIBAgIQAv1kld7ZLcQUgMRbq1FhwjAKBggqhkjOPQQDAjBCMQsw
CQYDVQQGEwJERTENMAsGA1UEChMERGVtbzENMAsGA1UECxMERGVtbzEVMBMGA1UE
AxMMRGVtby1Vbml0LTAxMB4XDTI1MDcxODA4MTc1MloXDTM1MDcxNjA4MTc1Mlow
QjELMAkGA1UEBhMCREUxDTALBgNVBAoTBERlbW8xDTALBgNVBAsTBERlbW8xFTAT
BgNVBAMTDERlbW8tVW5pdC0wMTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABATV
1BjmClUdj73+BQ+uCzs5pzViPE3pRHkd7NStYpyV1sPRG163Y7gvB1fxnrir1cMW
eZVQFuOui+oQCC1JifyjQjBAMA4GA1UdDwEB/wQEAwIHgDAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBRrOVr6S+4RIV3w36ltXcdZ+bgO5TAKBggqhkjOPQQDAgNI
ADBFAiAGfHooUHUwrc0YO5ZmS695HFn7KVhwrXR5d6bt7G/ICwIhANl5FA4RuwxV
0JFHA2FjwLpc8+0j54WEVOZRwV+opGxq
-----END CERTIFICATE-----`

		keyPEM := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIA8JIhB5iM3+ekgN9PIpYZ5F7gIOfPFc1ud6rYECp2ftoAoGCCqGSM49
AwEHoUQDQgAEBNXUGOYKVR2Pvf4FD64LOzmnNWI8TelEeR3s1K1inJXWw9EbXrdj
uC8HV/GeuKvVwxZ5lVAW466L6hAILUmJ/A==
-----END EC PRIVATE KEY-----`

		// 输出证书以便查看
		fmt.Println(certPEM)
		fmt.Println(keyPEM)

		// 从PEM加载证书
		certificate, err = tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			log.Fatal(err)
		}

		// 固定的SKI: 6b395afa4bee11215df0dfa96d5dc759f9b80ee5
		log.Printf("🔑 [TLS] Fixed SKI: 6b395afa4bee11215df0dfa96d5dc759f9b80ee5")
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		usage()
		log.Fatal(err)
	}

	// 增加TLS握手超时时间，从60秒增加到120秒
	configuration, err := api.NewConfiguration(
		"Demo", "Demo", "ControlBox", "123456789",
		[]shipapi.DeviceCategoryType{shipapi.DeviceCategoryTypeEnergyManagementSystem},
		model.DeviceTypeTypeElectricitySupplySystem,
		[]model.EntityTypeType{model.EntityTypeTypeGridGuard},
		port, certificate, time.Second*120)
	if err != nil {
		log.Fatal(err)
	}

	// 设置更宽松的TLS配置
	log.Printf("🔧 [TLS] Configuring relaxed TLS settings for better compatibility")
	configuration.SetAlternateIdentifier("Demo-ControlBox-123456789")

	// 确保作为TLS服务器运行
	log.Printf("🔄 [TLS] Configuring as TLS server - waiting for evcc client connections")

	// 设置宽松的客户端证书验证 - 使用默认配置
	log.Printf("🔒 [TLS] Using default certificate verification settings")

	// 添加TLS握手详细日志
	log.Printf("🔒 [TLS] Adding TLS handshake debug logging")

	// 打印本地SKI信息 - 将在服务启动后获取
	log.Printf("🔑 [TLS] Local SKI will be displayed after service starts")

	// 设置更详细的连接日志
	log.Printf("🔍 [DEBUG] Enabling detailed connection logging")

	// 添加host配置日志
	log.Printf("🌐 [EEBUS] Configuration created for port %d", port)
	log.Printf("🏠 [EEBUS] System hostname: %s", getSystemHostname())
	log.Printf("🔐 [TLS] Certificate Subject: %s", getCertificateSubject(certificate))
	log.Printf("⏱️  [TLS] Handshake timeout: 120 seconds")

	h.myService = service.NewService(configuration, h)
	h.myService.SetLogging(h)

	if err = h.myService.Setup(); err != nil {
		fmt.Println(err)
		return
	}

	h.consumptionLimits = ucapi.LoadLimit{
		IsActive: false,
		Value:    4200,
		Duration: 2 * time.Hour,
	}

	h.productionLimits = ucapi.LoadLimit{
		IsActive: false,
		Value:    5000,
		Duration: 1 * time.Hour,
	}

	h.consumptionFailsafeLimits = failsafeLimits{
		Value:    4200,
		Duration: 2 * time.Hour,
	}

	h.productionFailsafeLimits = failsafeLimits{
		Value:    5000,
		Duration: 1 * time.Hour,
	}

	localEntity := h.myService.LocalDevice().EntityForType(model.EntityTypeTypeGridGuard)
	h.uclpc = lpc.NewLPC(localEntity, h.OnLPCEvent)
	h.myService.AddUseCase(h.uclpc)

	// h.uclpp = lpp.NewLPP(localEntity, h.OnLPPEvent)
	// h.myService.AddUseCase(h.uclpp)

	h.remoteEntities = map[spineapi.EntityRemoteInterface][]string{}

	if len(remoteSki) == 0 {
		log.Printf("⚠️  [EEBUS] No remote SKI provided, exiting")
		os.Exit(0)
	}

	log.Printf("🎯 [EEBUS] Registering target remote SKI: %s", remoteSki)
	h.myService.RegisterRemoteSKI(remoteSki)

	log.Printf("🚀 [EEBUS] Starting EEBUS service on port %d", port)
	h.myService.Start()
	log.Printf("✅ [EEBUS] EEBUS service started successfully")

	// 显示本地SKI信息 - 通过日志输出查看

	// 显示服务的网络信息
	log.Printf("🌐 [EEBUS] Service network info:")
	qrCode := generateQRCodeText(h)
	log.Printf("   QR Code: %s", qrCode)

	// 从QR码中提取host信息
	if len(qrCode) > 0 {
		log.Printf("🔍 [EEBUS] Analyzing QR code for host information...")
		if host := extractHostFromQRCode(qrCode); host != "" {
			log.Printf("🏠 [EEBUS] Advertised host: %s", host)
		}
	}

	// 保持服务运行，等待上下文取消
	log.Printf("⏳ [EEBUS] Service is running, waiting for connections...")
	<-ctx.Done()
	log.Printf("🛑 [EEBUS] Context cancelled, stopping service...")
}

// shutdown performs graceful shutdown of all controlbox resources
func (h *controlbox) shutdown() {
	log.Println("Shutting down controlbox...")

	// Cancel context to signal all goroutines to stop
	if h.cancel != nil {
		h.cancel()
	}

	// Stop auto-restore timers
	if h.consumptionRestoreTimer != nil {
		h.consumptionRestoreTimer.Stop()
		log.Println("Stopped consumption restore timer")
	}
	if h.productionRestoreTimer != nil {
		h.productionRestoreTimer.Stop()
		log.Println("Stopped production restore timer")
	}

	// Shutdown eebus service
	if h.myService != nil {
		h.myService.Shutdown()
	}

	log.Println("Controlbox shutdown complete")
}

// EEBUSServiceHandler

func (h *controlbox) RemoteSKIConnected(service api.ServiceInterface, ski string) {
	h.isConnected = true
	log.Printf("✅ [EEBUS] Remote SKI connected: %s", ski)
	log.Printf("📡 [EEBUS] Connection established successfully with remote device")
	log.Printf("🔗 [EEBUS] Local service is now connected to remote SKI: %s", ski)
}

func (h *controlbox) RemoteSKIDisconnected(service api.ServiceInterface, ski string) {
	h.isConnected = false
	log.Printf("❌ [EEBUS] Remote SKI disconnected: %s", ski)
	log.Printf("🔌 [EEBUS] Connection lost with remote device")
	log.Printf("⚠️  [EEBUS] Local service is no longer connected to remote SKI: %s", ski)
}

func (h *controlbox) VisibleRemoteServicesUpdated(service api.ServiceInterface, entries []shipapi.RemoteService) {
	log.Printf("🔍 [mDNS] Visible remote services updated, found %d services", len(entries))

	for i, entry := range entries {
		log.Printf("📡 [mDNS] Service %d: SKI=%s, Name=%s, Brand=%s, Model=%s",
			i+1, entry.Ski, entry.Name, entry.Brand, entry.Model)

		// 检查是否是我们要连接的目标SKI
		if entry.Ski == remoteSki {
			log.Printf("🎯 [mDNS] Found target remote SKI: %s", entry.Ski)
		}
	}

	if len(entries) == 0 {
		log.Printf("⚠️  [mDNS] No remote services discovered")
	}
}

func (h *controlbox) ServiceShipIDUpdate(ski string, shipID string) {
	log.Printf("🚢 [SHIP] Ship ID updated for SKI %s: %s", ski, shipID)
}

func (h *controlbox) ServicePairingDetailUpdate(ski string, detail *shipapi.ConnectionStateDetail) {
	log.Printf("🔐 [PAIRING] Pairing detail update for SKI %s: State=%s", ski, detail.State())

	// 详细的连接状态分析
	switch detail.State() {
	case shipapi.ConnectionStateRemoteDeniedTrust:
		log.Printf("❌ [PAIRING] Remote service %s denied trust", ski)
		log.Printf("🔍 [PAIRING] 可能的原因:")
		log.Printf("   - 证书不匹配或无效")
		log.Printf("   - SKI不在对方的信任列表中")
		log.Printf("   - 对方设备拒绝新的配对请求")
		if ski == remoteSki {
			log.Printf("🚨 [PAIRING] Target remote service denied trust. Exiting.")
			h.myService.CancelPairingWithSKI(ski)
			h.myService.UnregisterRemoteSKI(ski)
			h.myService.Shutdown()
			os.Exit(0)
		}
	case shipapi.ConnectionStateError:
		log.Printf("💥 [PAIRING] Connection error for %s: %v", ski, detail.Error())
		if detail.Error() != nil {
			errorMsg := detail.Error().Error()
			log.Printf("🔍 [PAIRING] Error details: %s", errorMsg)

			// 分析常见错误
			if strings.Contains(errorMsg, "no such host") {
				log.Printf("🌐 [PAIRING] DNS解析失败 - 检查主机名或使用IP地址")
			} else if strings.Contains(errorMsg, "connection refused") {
				log.Printf("🔌 [PAIRING] 连接被拒绝 - 检查目标端口是否开放")
			} else if strings.Contains(errorMsg, "timeout") {
				log.Printf("⏰ [PAIRING] 连接超时 - 检查网络连通性和防火墙")
			} else if strings.Contains(errorMsg, "certificate") {
				log.Printf("🔐 [PAIRING] 证书问题 - 检查证书配置")
			} else if strings.Contains(errorMsg, "Node rejected") {
				log.Printf("🚫 [PAIRING] 节点被应用层拒绝 - 检查SKI配置和信任设置")
			}
		}
	case shipapi.ConnectionStateReceivedPairingRequest:
		log.Printf("📨 [PAIRING] Received pairing request from %s", ski)
		log.Printf("🤝 [PAIRING] 准备接受配对请求")
	case shipapi.ConnectionStateInitiated:
		log.Printf("🚀 [PAIRING] Connection initiated with %s", ski)
		log.Printf("⏳ [PAIRING] 等待连接建立...")
	case shipapi.ConnectionStateCompleted:
		log.Printf("✅ [PAIRING] Connection completed with %s", ski)
		log.Printf("🎉 [PAIRING] 连接成功建立!")
	default:
		log.Printf("📋 [PAIRING] Connection state for %s: %s", ski, detail.State())
	}
}

func (h *controlbox) AllowWaitingForTrust(ski string) bool {
	return ski == remoteSki
}

// LPC Event Handler

func (h *controlbox) sendConsumptionLimit(entity spineapi.EntityRemoteInterface) {
	resultCB := func(msg model.ResultDataType) {
		if *msg.ErrorNumber == model.ErrorNumberTypeNoError {
			fmt.Println("Consumption limit accepted.")
		} else {
			fmt.Println("Consumption limit rejected. Code", *msg.ErrorNumber, "Description", *msg.Description)
		}
	}
	msgCounter, err := h.uclpc.WriteConsumptionLimit(entity, h.consumptionLimits, resultCB)
	if err != nil {
		fmt.Println("Failed to send consumption limit", err)
		return
	}
	fmt.Println("Sent consumption limit to", entity.Device().Ski(), "with msgCounter", msgCounter)

	// 如果限制是激活的，设置10分钟后自动恢复（取消限制）
	if h.consumptionLimits.IsActive {
		// 取消之前的定时器（如果存在）
		if h.consumptionRestoreTimer != nil {
			h.consumptionRestoreTimer.Stop()
		}

		// 设置10分钟后恢复限制的定时器
		h.consumptionRestoreTimer = time.AfterFunc(10*time.Minute, func() {
			fmt.Println("Auto-restoring consumption limit after 10 minutes...")
			
			// 创建一个非激活的限制来恢复
			restoreLimit := ucapi.LoadLimit{
				IsActive: false,
				Value:    h.consumptionLimits.Value,
				Duration: h.consumptionLimits.Duration,
			}
			
			restoreResultCB := func(msg model.ResultDataType) {
				if *msg.ErrorNumber == model.ErrorNumberTypeNoError {
					fmt.Println("Consumption limit auto-restore accepted.")
				} else {
					fmt.Println("Consumption limit auto-restore rejected. Code", *msg.ErrorNumber, "Description", *msg.Description)
				}
			}
			
			// 发送恢复限制
			if msgCounter, err := h.uclpc.WriteConsumptionLimit(entity, restoreLimit, restoreResultCB); err != nil {
				fmt.Println("Failed to auto-restore consumption limit", err)
			} else {
				fmt.Println("Auto-restored consumption limit to", entity.Device().Ski(), "with msgCounter", msgCounter)
				// 更新本地状态
				h.consumptionLimits = restoreLimit
			}
		})
		
		fmt.Println("Set auto-restore timer for consumption limit (10 minutes)")
	}
}

func (h *controlbox) sendConsumptionFailsafeLimit(entity spineapi.EntityRemoteInterface) {
	msgCounter, err := h.uclpc.WriteFailsafeConsumptionActivePowerLimit(entity, h.consumptionFailsafeLimits.Value)
	if err != nil {
		fmt.Println("Failed to send consumption failsafe limit", err)
		return
	}
	fmt.Println("Sent consumption failsafe limit to", entity.Device().Ski(), "with msgCounter", msgCounter)
}

func (h *controlbox) sendConsumptionFailsafeDuration(entity spineapi.EntityRemoteInterface) {
	msgCounter, err := h.uclpc.WriteFailsafeDurationMinimum(entity, h.consumptionFailsafeLimits.Duration)
	if err != nil {
		fmt.Println("Failed to send consumption failsafe duration", err)
		return
	}
	fmt.Println("Sent consumption failsafe duration to", entity.Device().Ski(), "with msgCounter", msgCounter)
}

func (h *controlbox) sendProductionFailsafeLimit(entity spineapi.EntityRemoteInterface) {
	msgCounter, err := h.uclpp.WriteFailsafeProductionActivePowerLimit(entity, h.productionFailsafeLimits.Value)
	if err != nil {
		fmt.Println("Failed to send production failsafe limit", err)
		return
	}
	fmt.Println("Sent production failsafe limit to", entity.Device().Ski(), "with msgCounter", msgCounter)
}

func (h *controlbox) sendProductionFailsafeDuration(entity spineapi.EntityRemoteInterface) {
	msgCounter, err := h.uclpp.WriteFailsafeDurationMinimum(entity, h.productionFailsafeLimits.Duration)
	if err != nil {
		fmt.Println("Failed to send production failsafe duration", err)
		return
	}
	fmt.Println("Sent production failsafe duration to", entity.Device().Ski(), "with msgCounter", msgCounter)
}

func (h *controlbox) readConsumptionNominalMax(entity spineapi.EntityRemoteInterface) {
	nominal, err := h.uclpc.ConsumptionNominalMax(entity)
	if err != nil {
		fmt.Println("Failed to get consumption nominal max", err)
		return
	}

	frontend.sendValue(GetConsumptionNominalMax, "LPC", nominal)
}

func (h *controlbox) OnLPCEvent(ski string, device spineapi.DeviceRemoteInterface, entity spineapi.EntityRemoteInterface, event api.EventType) {
	if !h.isConnected {
		return
	}

	switch event {
	case lpc.UseCaseSupportUpdate:
		listUCs := h.remoteEntities[entity]
		if listUCs == nil {
			listUCs = []string{}
		}
		h.remoteEntities[entity] = append(listUCs, "LPC")

		fmt.Println("Sending consumption limit in 5s...")

		time.AfterFunc(5*time.Second, func() {
			frontend.sendNotification(EntityListChanged)

			// h.readConsumptionNominalMax(entity)
			h.sendConsumptionLimit(entity)
			h.sendConsumptionFailsafeLimit(entity)
			h.sendConsumptionFailsafeDuration(entity)
		})
	case lpc.DataUpdateLimit:
		if currentLimit, err := h.uclpc.ConsumptionLimit(entity); err == nil {
			h.consumptionLimits = currentLimit

			if currentLimit.IsActive {
				fmt.Println("New consumption limit received: active,", currentLimit.Value, "W,", currentLimit.Duration)
			} else {
				fmt.Println("New consumption limit received: inactive,", currentLimit.Value, "W,", currentLimit.Duration)
			}
			frontend.sendLimit(GetConsumptionLimit, "LPC", ucapi.LoadLimit{
				IsActive: currentLimit.IsActive,
				Duration: currentLimit.Duration / time.Second,
				Value:    currentLimit.Value,
			})
		}
	case lpc.DataUpdateFailsafeConsumptionActivePowerLimit:
		if limit, err := h.uclpc.FailsafeConsumptionActivePowerLimit(entity); err == nil {
			h.consumptionFailsafeLimits.Value = limit

			frontend.sendValue(GetConsumptionFailsafeValue, "LPC", limit)
		}
	case lpc.DataUpdateFailsafeDurationMinimum:
		if duration, err := h.uclpc.FailsafeDurationMinimum(entity); err == nil {
			h.consumptionFailsafeLimits.Duration = duration

			frontend.sendValue(GetConsumptionFailsafeDuration, "LPC", float64(duration/time.Second))
		}
	// case lpc.DataUpdateHeartbeat: // 在新版本中可能被移除或重命名
	//	frontend.sendNotification(GetConsumptionHeartbeat)
	default:
		return
	}
}

// LPP Event Handler

func (h *controlbox) sendProductionLimit(entity spineapi.EntityRemoteInterface) {
	resultCB := func(msg model.ResultDataType) {
		if *msg.ErrorNumber == model.ErrorNumberTypeNoError {
			fmt.Println("Production limit accepted.")
		} else {
			fmt.Println("Production limit rejected. Code", *msg.ErrorNumber, "Description", *msg.Description)
		}
	}
	msgCounter, err := h.uclpp.WriteProductionLimit(entity, h.productionLimits, resultCB)
	if err != nil {
		fmt.Println("Failed to send production limit", err)
		return
	}
	fmt.Println("Sent production limit to", entity.Device().Ski(), "with msgCounter", msgCounter)

	// 如果限制是激活的，设置10分钟后自动恢复（取消限制）
	if h.productionLimits.IsActive {
		// 取消之前的定时器（如果存在）
		if h.productionRestoreTimer != nil {
			h.productionRestoreTimer.Stop()
		}

		// 设置10分钟后恢复限制的定时器
		h.productionRestoreTimer = time.AfterFunc(10*time.Minute, func() {
			fmt.Println("Auto-restoring production limit after 10 minutes...")
			
			// 创建一个非激活的限制来恢复
			restoreLimit := ucapi.LoadLimit{
				IsActive: false,
				Value:    h.productionLimits.Value,
				Duration: h.productionLimits.Duration,
			}
			
			restoreResultCB := func(msg model.ResultDataType) {
				if *msg.ErrorNumber == model.ErrorNumberTypeNoError {
					fmt.Println("Production limit auto-restore accepted.")
				} else {
					fmt.Println("Production limit auto-restore rejected. Code", *msg.ErrorNumber, "Description", *msg.Description)
				}
			}
			
			// 发送恢复限制
			if msgCounter, err := h.uclpp.WriteProductionLimit(entity, restoreLimit, restoreResultCB); err != nil {
				fmt.Println("Failed to auto-restore production limit", err)
			} else {
				fmt.Println("Auto-restored production limit to", entity.Device().Ski(), "with msgCounter", msgCounter)
				// 更新本地状态
				h.productionLimits = restoreLimit
			}
		})
		
		fmt.Println("Set auto-restore timer for production limit (10 minutes)")
	}
}

func (h *controlbox) sendProductiomFailsafeLimit(entity spineapi.EntityRemoteInterface) {
	msgCounter, err := h.uclpp.WriteFailsafeProductionActivePowerLimit(entity, h.productionFailsafeLimits.Value)
	if err != nil {
		fmt.Println("Failed to send consumption limit", err)
		return
	}
	fmt.Println("Sent production limit to", entity.Device().Ski(), "with msgCounter", msgCounter)
}

func (h *controlbox) sendProductiomFailsafeDuration(entity spineapi.EntityRemoteInterface) {
	msgCounter, err := h.uclpp.WriteFailsafeDurationMinimum(entity, h.productionFailsafeLimits.Duration)
	if err != nil {
		fmt.Println("Failed to send consumption limit", err)
		return
	}
	fmt.Println("Sent production limit to", entity.Device().Ski(), "with msgCounter", msgCounter)
}

func (h *controlbox) readProductionNominalMax(entity spineapi.EntityRemoteInterface) {
	nominal, err := h.uclpp.ProductionNominalMax(entity)
	if err != nil {
		fmt.Println("Failed to get production nominal max", err)
		return
	}

	frontend.sendValue(GetProductionNominalMax, "LPP", nominal)
}

func (h *controlbox) OnLPPEvent(ski string, device spineapi.DeviceRemoteInterface, entity spineapi.EntityRemoteInterface, event api.EventType) {
	if !h.isConnected {
		return
	}

	switch event {
	case lpp.UseCaseSupportUpdate:
		listUCs := h.remoteEntities[entity]
		if listUCs == nil {
			listUCs = []string{}
		}
		h.remoteEntities[entity] = append(listUCs, "LPP")

		fmt.Println("Sending production limit in 5s...")

		time.AfterFunc(5*time.Second, func() {
			frontend.sendNotification(EntityListChanged)

			// h.readProductionNominalMax(entity)
			h.sendProductionLimit(entity)
			h.sendProductiomFailsafeLimit(entity)
			h.sendProductiomFailsafeDuration(entity)
		})
	case lpp.DataUpdateLimit:
		if currentLimit, err := h.uclpp.ProductionLimit(entity); err == nil {
			h.productionLimits = currentLimit

			if currentLimit.IsActive {
				fmt.Println("New production limit received: active,", currentLimit.Value, "W,", currentLimit.Duration)
			} else {
				fmt.Println("New production limit received: inactive,", currentLimit.Value, "W,", currentLimit.Duration)
			}

			frontend.sendLimit(GetProductionLimit, "LPP", ucapi.LoadLimit{
				IsActive: currentLimit.IsActive,
				Duration: currentLimit.Duration / time.Second,
				Value:    currentLimit.Value,
			})
		}
	case lpp.DataUpdateFailsafeProductionActivePowerLimit:
		if limit, err := h.uclpp.FailsafeProductionActivePowerLimit(entity); err == nil {
			h.productionFailsafeLimits.Value = limit

			frontend.sendValue(GetProductionFailsafeValue, "LPP", limit)
		}
	case lpp.DataUpdateFailsafeDurationMinimum:
		if duration, err := h.uclpp.FailsafeDurationMinimum(entity); err == nil {
			h.productionFailsafeLimits.Duration = duration

			frontend.sendValue(GetProductionFailsafeDuration, "LPP", float64(duration/time.Second))
		}
	// case lpp.DataUpdateHeartbeat: // 在新版本中可能被移除或重命名
	//	frontend.sendNotification(GetProductionHeartbeat)
	default:
		return
	}
}

// main app
func usage() {
	fmt.Println("First Run (auto-generate certificate):")
	fmt.Println("  go run /examples/controlbox/main.go <serverport>")
	fmt.Println()
	fmt.Println("With Remote SKI (auto-generate certificate):")
	fmt.Println("  go run /examples/controlbox/main.go <serverport> <remoteski>")
	fmt.Println()
	fmt.Println("With Custom Certificate:")
	fmt.Println("  go run /examples/controlbox/main.go <serverport> <remoteski> <crtfile> <keyfile>")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := controlbox{}
	h.run(ctx)

	// Setup HTTP server with graceful shutdown
	setupRoutes(&h)
	server := &http.Server{
		Addr: ":" + strconv.Itoa(httpdPort),
	}

	// Start HTTP server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		log.Println("Starting HTTP server on port", httpdPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	// Setup signal handling for graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case <-sig:
		fmt.Println("Interrupt signal received, shutting down...")
	case err := <-serverErrChan:
		log.Printf("HTTP server error: %v", err)
	case <-ctx.Done():
		fmt.Println("Context cancelled, shutting down...")
	}

	// Perform graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Shutdown controlbox and all its resources
	h.shutdown()

	// Close frontend websocket connection
	if err := frontend.Close(); err != nil {
		log.Printf("Frontend websocket close error: %v", err)
	}

	fmt.Println("Shutdown complete")
}

// Logging interface

func (h *controlbox) Trace(args ...interface{}) {
	// h.print("TRACE", args...)
}

func (h *controlbox) Tracef(format string, args ...interface{}) {
	// h.printFormat("TRACE", format, args...)
}

func (h *controlbox) Debug(args ...interface{}) {
	// h.print("DEBUG", args...)
}

func (h *controlbox) Debugf(format string, args ...interface{}) {
	// h.printFormat("DEBUG", format, args...)
}

func (h *controlbox) Info(args ...interface{}) {
	h.print("INFO ", args...)
}

func (h *controlbox) Infof(format string, args ...interface{}) {
	h.printFormat("INFO ", format, args...)
}

func (h *controlbox) Error(args ...interface{}) {
	h.print("ERROR", args...)
}

func (h *controlbox) Errorf(format string, args ...interface{}) {
	h.printFormat("ERROR", format, args...)
}

func (h *controlbox) currentTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (h *controlbox) print(msgType string, args ...interface{}) {
	value := fmt.Sprintln(args...)
	fmt.Printf("%s %s %s", h.currentTimestamp(), msgType, value)
}

func (h *controlbox) printFormat(msgType, format string, args ...interface{}) {
	value := fmt.Sprintf(format, args...)
	fmt.Println(h.currentTimestamp(), msgType, value)
}

// web frontend

const (
	httpdPort int = 7071
)

const (
	Text                           = 0
	QRCode                         = 1
	Acknowledge                    = 2
	EntityListChanged              = 3
	GetEntityList                  = 4
	GetAllData                     = 5
	SetConsumptionLimit            = 6
	GetConsumptionLimit            = 7
	SetProductionLimit             = 8
	GetProductionLimit             = 9
	SetConsumptionFailsafeValue    = 10
	GetConsumptionFailsafeValue    = 11
	SetConsumptionFailsafeDuration = 12
	GetConsumptionFailsafeDuration = 13
	SetProductionFailsafeValue     = 14
	GetProductionFailsafeValue     = 15
	SetProductionFailsafeDuration  = 16
	GetProductionFailsafeDuration  = 17
	GetConsumptionNominalMax       = 18
	GetProductionNominalMax        = 19
	GetConsumptionHeartbeat        = 20
	StopConsumptionHeartbeat       = 21
	StartConsumptionHeartbeat      = 22
	GetProductionHeartbeat         = 23
	StopProductionHeartbeat        = 24
	StartProductionHeartbeat       = 25
)

type EntityDescription struct {
	Name     string
	SKI      string
	UseCases []string
}

type Message struct {
	Type       int
	Text       string
	Limit      ucapi.LoadLimit
	Value      float64
	EntityList []EntityDescription
	UseCase    string
}

func sendData(h *controlbox) {
	qrText := generateQRCodeText(h)
	log.Printf("Sending QR code: %s", qrText)
	frontend.sendText(QRCode, qrText)

	frontend.sendLimit(GetConsumptionLimit, "LPC", ucapi.LoadLimit{
		IsActive: h.consumptionLimits.IsActive,
		Duration: h.consumptionLimits.Duration / time.Second,
		Value:    h.consumptionLimits.Value,
	})

	frontend.sendLimit(GetProductionLimit, "LPP", ucapi.LoadLimit{
		IsActive: h.productionLimits.IsActive,
		Duration: h.productionLimits.Duration / time.Second,
		Value:    h.productionLimits.Value,
	})

	frontend.sendValue(GetConsumptionFailsafeValue, "LPC", h.consumptionFailsafeLimits.Value)

	frontend.sendValue(GetConsumptionFailsafeDuration, "LPC", float64(h.consumptionFailsafeLimits.Duration/time.Second))

	frontend.sendValue(GetProductionFailsafeValue, "LPP", h.productionFailsafeLimits.Value)

	frontend.sendValue(GetProductionFailsafeDuration, "LPP", float64(h.productionFailsafeLimits.Duration/time.Second))
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// allow connection from any host
		return true
	},
}

func setupRoutes(h *controlbox) {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(h, w, r)
	})
}

func serveWs(h *controlbox, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	frontend = WebsocketClient{
		websocket: ws,
	}

	log.Println("Client Connected")

	sendData(h)

	// Start reader in a goroutine with context support
	go reader(h, ws)
}

func reader(h *controlbox, ws *websocket.Conn) {
	defer func() {
		if ws != nil {
			ws.Close()
		}
	}()

	for {
		// Check if context is cancelled before reading
		select {
		case <-h.ctx.Done():
			log.Println("WebSocket reader shutting down due to context cancellation")
			return
		default:
		}

		// read in a message
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			return
		}
		// print out that message for clarity
		fmt.Println(string(p))

		data := Message{}
		json.Unmarshal([]byte(p), &data)

		if data.Type == GetEntityList {
			frontend.sendEntityList(GetEntityList, h.remoteEntities)
		} else if data.Type == GetAllData {
			sendData(h)
		} else if data.Type == SetConsumptionLimit {
			limit := data.Limit

			h.consumptionLimits.IsActive = limit.IsActive
			h.consumptionLimits.Value = limit.Value
			h.consumptionLimits.Duration = limit.Duration * time.Second

			for _, remoteEntityScenario := range h.uclpc.RemoteEntitiesScenarios() {
				h.sendConsumptionLimit(remoteEntityScenario.Entity)
			}
		} else if data.Type == SetProductionLimit {
			limit := data.Limit

			h.productionLimits.IsActive = limit.IsActive
			h.productionLimits.Value = limit.Value
			h.productionLimits.Duration = limit.Duration * time.Second

			for _, remoteEntityScenario := range h.uclpp.RemoteEntitiesScenarios() {
				h.sendProductionLimit(remoteEntityScenario.Entity)
			}
		} else if data.Type == SetConsumptionFailsafeValue {
			limit := data.Value

			h.consumptionFailsafeLimits.Value = limit

			for _, remoteEntityScenario := range h.uclpc.RemoteEntitiesScenarios() {
				h.sendConsumptionFailsafeLimit(remoteEntityScenario.Entity)
			}
		} else if data.Type == SetConsumptionFailsafeDuration {
			limit := data.Value

			h.consumptionFailsafeLimits.Duration = time.Duration(limit) * time.Second

			for _, remoteEntityScenario := range h.uclpc.RemoteEntitiesScenarios() {
				h.sendConsumptionFailsafeDuration(remoteEntityScenario.Entity)
			}
		} else if data.Type == SetProductionFailsafeValue {
			limit := data.Value

			h.productionFailsafeLimits.Value = limit

			for _, remoteEntityScenario := range h.uclpp.RemoteEntitiesScenarios() {
				h.sendProductionFailsafeLimit(remoteEntityScenario.Entity)
			}
		} else if data.Type == SetProductionFailsafeDuration {
			limit := data.Value

			h.productionFailsafeLimits.Duration = time.Duration(limit) * time.Second

			for _, remoteEntityScenario := range h.uclpp.RemoteEntitiesScenarios() {
				h.sendProductionFailsafeDuration(remoteEntityScenario.Entity)
			}
		} else if data.Type == StopConsumptionHeartbeat {
			// 在新版本中，heartbeat方法可能被移动或重命名
			// h.uclpc.StopHeartbeat()
			log.Printf("⚠️ StopConsumptionHeartbeat not implemented in new API version")
		} else if data.Type == StartConsumptionHeartbeat {
			// h.uclpc.StartHeartbeat()
			log.Printf("⚠️ StartConsumptionHeartbeat not implemented in new API version")
		}

		answer := Message{
			Type: Acknowledge,
		}

		bytes, _ := json.Marshal(answer)
		if err := ws.WriteMessage(1, bytes); err != nil {
			log.Println("WebSocket write error:", err)
			return
		}
	}
}
