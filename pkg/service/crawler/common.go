package crawler

import (
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"net/http"
	"sync"
)

type VietStockCrawler struct {
	client    *http.Client
	mutex     sync.RWMutex
	isRunning bool
}

type CrawlResult struct {
	Stocks []modelssvc.VN30Stock `json:"stocks"`
	Error  error                 `json:"error,omitempty"`
	Count  int                   `json:"count"`
	Source string                `json:"source"`
}

// VN30 Stock Symbol Constants - cập nhật theo danh sách VN30 mới nhất
const (
	// Banking sector
	VN30SymbolACB = "ACB" // Ngân hàng Á Châu
	VN30SymbolBID = "BID" // Ngân hàng Đầu tư và Phát triển
	VN30SymbolCTG = "CTG" // Ngân hàng Công thương
	VN30SymbolHDB = "HDB" // Ngân hàng Phát triển TP.HCM
	VN30SymbolMBB = "MBB" // Ngân hàng Quân đội
	VN30SymbolSHB = "SHB" // Ngân hàng Sài Gòn - Hà Nội
	VN30SymbolSSB = "SSB" // Ngân hàng Đông Nam Á
	VN30SymbolSTB = "STB" // Ngân hàng Sài Gòn Thương Tín
	VN30SymbolTCB = "TCB" // Ngân hàng Kỹ thương
	VN30SymbolTPB = "TPB" // Ngân hàng Tiên Phong
	VN30SymbolVCB = "VCB" // Ngân hàng Ngoại thương
	VN30SymbolVPB = "VPB" // Ngân hàng Việt Nam Thịnh vượng

	// Real Estate & Construction
	VN30SymbolBCM = "BCM" // Khoáng sản Bình Dương
	VN30SymbolHPG = "HPG" // Hoà Phát Group
	VN30SymbolVHM = "VHM" // Vinhomes
	VN30SymbolVIC = "VIC" // Vingroup
	VN30SymbolVRE = "VRE" // Vincom Retail

	// Technology & Telecom
	VN30SymbolFPT = "FPT" // FPT Corporation
	VN30SymbolVTI = "VTI" // Vinh Tien Investment

	// Energy & Oil Gas
	VN30SymbolGAS = "GAS" // Tổng Công ty Khí Việt Nam
	VN30SymbolGVR = "GVR" // Tập đoàn Cao su Việt Nam
	VN30SymbolPLX = "PLX" // Xăng dầu Petrolimex
	VN30SymbolPOW = "POW" // PetroVietnam Power

	// Consumer & Retail
	VN30SymbolMSN = "MSN" // Masan Group
	VN30SymbolMWG = "MWG" // Mobile World
	VN30SymbolSAB = "SAB" // Sabeco
	VN30SymbolVNM = "VNM" // Vinamilk

	// Financial Services
	VN30SymbolBVH = "BVH" // Bảo Việt Holdings
	VN30SymbolSSI = "SSI" // SSI Securities

	// Aviation
	VN30SymbolVJC = "VJC" // VietJet Air
)

// VN30Symbols - danh sách tất cả mã VN30 để dễ iterate
var VN30Symbols = []string{
	VN30SymbolACB, VN30SymbolBCM, VN30SymbolBID, VN30SymbolBVH, VN30SymbolCTG,
	VN30SymbolFPT, VN30SymbolGAS, VN30SymbolGVR, VN30SymbolHDB, VN30SymbolHPG,
	VN30SymbolMBB, VN30SymbolMSN, VN30SymbolMWG, VN30SymbolPLX, VN30SymbolPOW,
	VN30SymbolSAB, VN30SymbolSHB, VN30SymbolSSB, VN30SymbolSSI, VN30SymbolSTB,
	VN30SymbolTCB, VN30SymbolTPB, VN30SymbolVCB, VN30SymbolVHM, VN30SymbolVIC,
	VN30SymbolVJC, VN30SymbolVNM, VN30SymbolVPB, VN30SymbolVRE, VN30SymbolVTI,
}

// VN30SymbolNames - mapping từ symbol sang tên công ty
var VN30SymbolNames = map[string]string{
	VN30SymbolACB: "Ngân hàng Á Châu",
	VN30SymbolBCM: "Khoáng sản Bình Dương",
	VN30SymbolBID: "Ngân hàng Đầu tư và Phát triển",
	VN30SymbolBVH: "Bảo Việt Holdings",
	VN30SymbolCTG: "Ngân hàng Công thương",
	VN30SymbolFPT: "Tập đoàn FPT",
	VN30SymbolGAS: "Tổng Công ty Khí Việt Nam",
	VN30SymbolGVR: "Tập đoàn Cao su Việt Nam",
	VN30SymbolHDB: "Ngân hàng Phát triển TP.HCM",
	VN30SymbolHPG: "Hoà Phát Group",
	VN30SymbolMBB: "Ngân hàng Quân đội",
	VN30SymbolMSN: "Masan Group",
	VN30SymbolMWG: "Mobile World",
	VN30SymbolPLX: "Xăng dầu Petrolimex",
	VN30SymbolPOW: "PetroVietnam Power",
	VN30SymbolSAB: "Sabeco",
	VN30SymbolSHB: "Ngân hàng Sài Gòn - Hà Nội",
	VN30SymbolSSB: "Ngân hàng Đông Nam Á",
	VN30SymbolSSI: "SSI Securities",
	VN30SymbolSTB: "Ngân hàng Sài Gòn Thương Tín",
	VN30SymbolTCB: "Ngân hàng Kỹ thương",
	VN30SymbolTPB: "Ngân hàng Tiên Phong",
	VN30SymbolVCB: "Ngân hàng Ngoại thương",
	VN30SymbolVHM: "Vinhomes",
	VN30SymbolVIC: "Vingroup",
	VN30SymbolVJC: "VietJet Air",
	VN30SymbolVNM: "Công ty Cổ phần Sữa Việt Nam",
	VN30SymbolVPB: "Ngân hàng Việt Nam Thịnh vượng",
	VN30SymbolVRE: "Vincom Retail",
	VN30SymbolVTI: "Vinh Tien Investment",
}

var crawler *VietStockCrawler
