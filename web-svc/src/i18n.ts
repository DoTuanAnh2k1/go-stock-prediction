export type Lang = 'vi' | 'en';

export const translations: Record<Lang, Translations> = {
  vi: {
    // Navigation
    nav: {
      overview: 'Tổng quan',
      markets: 'Thị trường',
      simulation: 'Simulation',
      support: 'Hỗ trợ',
      guide: 'Hướng dẫn',
      settings: 'Cài đặt',
      users: 'Người dùng',
      leaderboard: 'Leaderboard',
      monitoring: 'Giám sát dữ liệu',
      marketGroups: 'Nhóm thị trường',
      commands: 'Lệnh',
      commandGroups: 'Nhóm lệnh',
      docs: 'Tài liệu',
    },
    // Market sub-items
    marketSubs: {
      overview: 'Tổng quan',
      predictions: 'Dự đoán',
      training: 'Huấn luyện',
    },
    // Market labels
    markets: {
      gold: 'Vàng',
    },
    // MobNav labels
    mob: {
      overview: 'Tổng quan',
      gold: 'Vàng',
      guide: 'Hướng dẫn',
    },
    // Topbar
    topbar: {
      searchPlaceholder: 'Tìm mã CK, ngành...',
      notifications: 'Thông báo',
      lightTheme: 'Sáng',
      darkTheme: 'Tối',
      logout: 'Đăng xuất',
      login: 'Đăng nhập',
    },
    // Page titles [display, breadcrumb]
    titles: {
      '/': ['Tổng quan thị trường', 'DASHBOARD'],
      '/markets/gold': ['Vàng', 'GOLD'],
      '/markets/gold/predictions': ['Dự đoán Vàng', 'PREDICTIONS · GOLD'],
      '/markets/gold/training': ['Huấn luyện Vàng', 'TRAINING · GOLD'],
      '/markets/crypto': ['Cryptocurrency', 'CRYPTO'],
      '/markets/nasdaq100': ['NASDAQ 100', 'NASDAQ 100 · US EQUITIES'],
      '/markets/nasdaq100/predictions': ['Dự đoán NASDAQ', 'PREDICTIONS · NASDAQ'],
      '/markets/nasdaq100/training': ['Huấn luyện NASDAQ', 'TRAINING · NASDAQ'],
      '/markets/sp500': ['S&P 500', 'S&P 500 · US EQUITIES'],
      '/markets/sp500/predictions': ['Dự đoán S&P 500', 'PREDICTIONS · S&P 500'],
      '/markets/sp500/training': ['Huấn luyện S&P 500', 'TRAINING · S&P 500'],
      '/markets/crypto/predictions': ['Dự đoán Crypto', 'PREDICTIONS · CRYPTO'],
      '/markets/crypto/training': ['Huấn luyện Crypto', 'TRAINING · CRYPTO'],
      '/simulation': ['Simulation Leaderboard', 'SIMULATION · LEADERBOARD'],
      '/guide': ['Hướng dẫn sử dụng', 'USER GUIDE'],
      '/settings': ['Cài đặt', 'SETTINGS'],
      '/admin/users': ['Quản lý người dùng', 'ADMIN · USERS'],
      '/admin/market-groups': ['Nhóm thị trường', 'ADMIN · MARKET GROUPS'],
      '/gold': ['Giá vàng', 'GOLD'],
      '/crypto': ['Cryptocurrency', 'CRYPTO'],
      '/nasdaq': ['NASDAQ 100', 'NASDAQ 100 · US EQUITIES'],
      '/monitoring': ['Giám sát dữ liệu', 'DATA PIPELINE · MONITORING'],
      '/admin/commands': ['Lệnh CLI', 'ADMIN · COMMANDS'],
      '/admin/command-groups': ['Nhóm lệnh CLI', 'ADMIN · COMMAND GROUPS'],
      '/admin/docs': ['Tài liệu kỹ thuật', 'ADMIN · DOCS'],
    },
    // Sidebar footer
    footer: {
      liveData: 'Dữ liệu trực tiếp',
      loadingData: 'Đang tải dữ liệu…',
      sampleData: 'Dữ liệu mẫu',
      dataSession: 'Dữ liệu · phiên gần nhất',
    },
    // ErrorBoundary
    error: {
      cannotDisplay: 'Không thể hiển thị trang này với dữ liệu hiện tại.',
      retry: 'Thử lại',
    },
    // TweaksPanel (in App.tsx)
    tweaks: {
      interface: 'Giao diện',
      accentColor: 'Màu nhấn',
      density: 'Mật độ',
      fontSize: 'Cỡ chữ',
      ticker: 'Thanh ticker',
    },
    // MarketTabs (shared across Crypto, Nasdaq, SP500, MarketPredictions, MarketTraining, MarketDetail)
    marketTabs: {
      overview: 'Tổng quan',
      predictions: 'Dự đoán',
      detail: 'Chi tiết',
      training: 'Huấn luyện',
      session: 'Phiên',
    },
    // Banner thị trường đóng cửa (NASDAQ / SP500)
    marketClosed: {
      weekend: 'Thị trường đang đóng cửa (cuối tuần). Crawl, dự đoán và bot tạm dừng cho tới phiên giao dịch kế tiếp.',
      holiday: 'Thị trường đang đóng cửa (ngày lễ Mỹ). Crawl, dự đoán và bot tạm dừng cho tới phiên giao dịch kế tiếp.',
    },
    // Date range labels
    dateRange: {
      today: 'H.nay',
      d7: '7N',
      d30: '30N',
      d90: '90N',
      d180: '180N',
      d365: '1Năm',
    },
    // Common shared strings
    common: {
      loading: 'Đang tải...',
      noData: 'Chưa có dữ liệu',
      noDataCollect: 'Chưa có dữ liệu. Hãy thu thập dữ liệu trước.',
      collect: 'Thu thập',
      predict: 'Dự đoán',
      refresh: 'Làm mới',
      exportCsv: 'Xuất CSV',
      allAlgos: 'Tất cả thuật toán',
      allSymbols: 'Tất cả mã',
      all: 'Tất cả',
      symbol: 'Mã',
      coin: 'Coin',
      current: 'Hiện tại',
      predicted: 'Dự đoán',
      actual: 'Thực tế',
      deviation: 'Lệch',
      accuracy: 'Độ CX',
      confidence: 'Tin cậy',
      date: 'Ngày',
      status: 'TT',
      noDataChart: 'Chưa có dữ liệu biểu đồ. Hãy thu thập dữ liệu trước.',
      loadingChart: 'Đang tải biểu đồ...',
      noCompareData: 'Chưa có dữ liệu so sánh dự đoán',
      predVsActual: 'Dự đoán vs Thực tế',
      latestPredResults: 'Kết quả dự đoán gần nhất',
      noConfirmedResults: 'Chưa có kết quả đã xác nhận. Kết quả sẽ xuất hiện sau khi dự đoán được đối chiếu với giá thực tế.',
      noPredictions: 'Chưa có dự đoán. Hãy chạy dự đoán trước.',
      noDataForItem: 'Không có dữ liệu cho mục này.',
      source: 'Nguồn',
      product: 'Sản phẩm',
      chartType: { line: 'Đường', candle: 'Nến' },
    },
    // Dashboard page
    dashboard: {
      liquidity: 'Thanh khoản',
      stocksMatched: 'cổ phiếu khớp lệnh',
      ensAccuracy: 'Độ chính xác ENS',
      ensModel: 'mô hình tổng hợp',
      indexChart: 'Diễn biến chỉ số',
      sessionToday: 'Phiên hôm nay · đóng cửa',
      noIndexData: 'Chưa có dữ liệu chỉ số',
      marketBreadth: 'Độ rộng thị trường',
      watchlist: 'Danh sách theo dõi',
      noWatchData: 'Chưa có dữ liệu. Đang tải...',
      colSymbol: 'Mã',
      colPrice: 'Giá',
      colChangePct: '±%',
      colVolume: 'KL (M)',
      col7Sessions: '7 phiên',
      mlModelStatus: 'Trạng thái mô hình ML',
      algorithms: 'thuật toán',
      noTrainingData: 'Chưa có dữ liệu huấn luyện',
      latestPredictions: 'Dự đoán mới nhất',
      noPredData: 'Chưa có dự đoán. Đang tải...',
      colCurrentPrice: 'Giá hiện tại',
      colPredPrice: 'Giá dự đoán',
      colExpectedDelta: 'Δ dự kiến',
      colAlgo: 'Thuật toán',
      colConfidence: 'Độ tin cậy',
      colTargetSession: 'Phiên mục tiêu',
      algoAccuracy: 'So sánh độ chính xác thuật toán',
      last30Sessions: '30 phiên',
      predByDay: 'Số dự đoán theo ngày',
      last7Days: '7 ngày',
      noChartData: 'Chưa có dữ liệu',
      predCount: 'dự đoán',
      topSimBots: 'Top Simulation Bots',
      topBotsDesc: '3 bots hiệu suất cao nhất',
      viewAll: 'Xem tất cả →',
      noSimData: 'Chưa có dữ liệu simulation.',
      runBacktest: 'Chạy backtest',
      up: 'Tăng',
      down: 'Giảm',
      ref: 'Tham chiếu',
      rising: 'tăng giá',
      leadingSectors: 'Ngành dẫn dắt',
      noMarketData: 'Chưa có dữ liệu thị trường',
      totalPredictions: 'Tổng dự đoán',
      allTime: 'toàn thời gian',
      predsToday: 'Dự đoán hôm nay',
      acrossMarkets: 'tất cả thị trường',
      activeBots: 'Bots hoạt động',
      inSimulation: 'đang chạy simulation',
      marketStatus: 'Trạng thái Pipeline',
      marketStatusDesc: 'Freshness dữ liệu & hoạt động dự đoán',
      lastCrawl: 'Crawl cuối',
      predsCount: 'dự đoán',
      loginForStatus: 'Đăng nhập để xem trạng thái pipeline hệ thống',
      loadingStatus: 'Đang tải trạng thái...',
      marketOpen: 'Mở cửa',
      marketClosed: 'Đóng cửa',
      fresh: 'Tươi',
      stale: 'Cũ',
      never: 'Chưa có',
      missing: 'thiếu',
      dirAccuracy: 'Độ chính xác dự đoán hướng',
      dirAccuracySub: 'TB các thị trường · từ lịch sử reconcile',
      dirAccuracyChart: 'So sánh direction accuracy',
      loginToSeeAccuracy: 'Đăng nhập để xem direction accuracy thực từ DB',
      noReconcileData: 'Chưa có dữ liệu reconcile',
      ensDir: 'ENS Direction Acc',
      ensDirSub: 'avg các thị trường',
    },
    // Crypto page
    crypto: {
      priceChart: 'Biểu đồ giá Crypto',
      noAccess: 'Bạn không có quyền truy cập thị trường Crypto.',
      loadingData: 'Đang tải dữ liệu crypto...',
      noDataKpi: 'Chưa có dữ liệu',
      tomorrowPred: 'Dự đoán Crypto phiên mai',
      noPredForCoin: 'Không có dữ liệu cho coin này.',
      marketInfo: 'Thông tin thị trường Crypto',
      collectRequest: 'Đã gửi yêu cầu thu thập Crypto',
      collectFail: 'Không thể gửi yêu cầu thu thập',
      predictRequest: 'Đã gửi yêu cầu chạy dự đoán Crypto',
      predictFail: 'Không thể gửi yêu cầu dự đoán',
      noChartData: 'Chưa có dữ liệu biểu đồ. Hãy thu thập dữ liệu trước.',
      noConfirmedForCoin: 'Không có dữ liệu cho coin này.',
      noCompareData: 'Chưa có dữ liệu so sánh dự đoán cho',
    },
    // Gold page
    gold: {
      priceChart: 'Biểu đồ giá vàng',
      noAccess: 'Bạn không có quyền truy cập thị trường Vàng.',
      loadingData: 'Đang tải dữ liệu vàng...',
      loading: 'đang tải...',
      sellPrice: 'giá bán',
      providerComparison: 'So sánh giá các nhà cung cấp',
      colProvider: 'Nhà cung cấp',
      colType: 'Loại',
      colRegion: 'Khu vực',
      colBuyPrice: 'Giá mua',
      colSellPrice: 'Giá bán',
      colSpread: 'Chênh lệch',
      tomorrowPred: 'Dự đoán giá vàng phiên mai',
      runPrediction: 'Chạy dự đoán',
      historicalBacktest: 'Backtest lịch sử',
      noPredForType: 'Không có dữ liệu cho loại này.',
      noGoldPred: 'Chưa có dự đoán giá vàng',
      collectRequest: 'Đã gửi yêu cầu thu thập giá vàng',
      collectFail: 'Chưa gọi được API thu thập',
      predictRequest: 'Đã gửi yêu cầu chạy dự đoán vàng',
      predictFail: 'Chưa gọi được API dự đoán',
      backtestRequest: 'Đang chạy gold backtest...',
      backtestFail: 'Chưa gọi được API backtest',
      sjcDetail: 'Chi tiết SJC 1 Lượng',
      last10Days: '10 ngày gần nhất',
      colDate: 'Ngày',
      colBuyVnd: 'Giá mua (VND)',
      colSellVnd: 'Giá bán (VND)',
      colSpreadLabel: 'Chênh lệch',
      noHistData: 'Chưa có dữ liệu lịch sử',
      noGoldHistory: 'Chưa có lịch sử giá vàng. Bấm thu thập để lấy data.',
      noConfirmedForType: 'Không có dữ liệu cho loại này.',
      noCompareData: 'Chưa có dữ liệu so sánh dự đoán',
      subEnsemble: 'BTMC SJC · Ensemble',
    },
    // Nasdaq page
    nasdaq: {
      priceChart: 'Biểu đồ giá NASDAQ 100',
      noAccess: 'Bạn không có quyền truy cập thị trường NASDAQ.',
      loadingData: 'Đang tải dữ liệu NASDAQ 100...',
      noDataKpi: 'Chưa có dữ liệu',
      tomorrowPred: 'Dự đoán NASDAQ phiên mai',
      priceTable: 'Bảng giá NASDAQ 100',
      collectRequest: 'Đã gửi yêu cầu thu thập NASDAQ',
      collectFail: 'Không thể gửi yêu cầu thu thập',
      predictRequest: 'Đã gửi yêu cầu chạy dự đoán NASDAQ',
      predictFail: 'Không thể gửi yêu cầu dự đoán',
      noChartData: 'Chưa có dữ liệu biểu đồ. Hãy thu thập dữ liệu trước.',
      noDataForSymbol: 'Không có dữ liệu cho mã này.',
      allAlgos: 'Tất cả thuật toán',
    },
    // SP500 page
    sp500: {
      priceChart: 'Biểu đồ giá S&P 500',
      noAccess: 'Bạn không có quyền truy cập thị trường S&P 500.',
      loadingData: 'Đang tải dữ liệu S&P 500...',
      noDataKpi: 'Chưa có dữ liệu',
      tomorrowPred: 'Dự đoán S&P 500 phiên mai',
      priceTable: 'Bảng giá S&P 500',
      collectRequest: 'Đã gửi yêu cầu thu thập S&P 500',
      collectFail: 'Không thể gửi yêu cầu thu thập',
      predictRequest: 'Đã gửi yêu cầu chạy dự đoán S&P 500',
      predictFail: 'Không thể gửi yêu cầu dự đoán',
      noChartData: 'Chưa có dữ liệu biểu đồ. Hãy thu thập dữ liệu trước.',
      noDataForSymbol: 'Không có dữ liệu cho mã này.',
      noConfirmed: 'Chưa có kết quả đã xác nhận. Kết quả sẽ xuất hiện sau khi dự đoán được đối chiếu với giá thực tế.',
    },
    // Simulation page
    simulation: {
      totalBots: 'Tổng số bots',
      monitored: 'đang được theo dõi',
      bestMarket: 'Thị trường tốt nhất',
      byAvgReturn: 'theo avg return',
      bestAlgo: 'Thuật toán tốt nhất',
      avgPerformance: 'avg performance',
      avgReturn: 'Avg Return',
      loadingData: 'Đang tải dữ liệu Simulation...',
      cannotLoad: 'Không thể tải dữ liệu. Backend chưa có dữ liệu simulation.',
      topReturnBots: 'Top bots theo Return %',
      highest: 'cao nhất',
      marketDist: 'Phân bố thị trường',
      byBotCount: 'theo số bot',
      leaderboard: 'Bảng xếp hạng Simulation',
      bots: 'bots',
      allAlgos: 'Tất cả thuật toán',
      runAllBacktest: 'Đã gửi yêu cầu chạy backtest tất cả bots',
      cannotSendRequest: 'Không thể gửi yêu cầu',
      noSimData: 'Chưa có dữ liệu simulation. Hãy chạy backtest trước.',
      runNow: 'Chạy backtest ngay',
      searchBot: 'Tìm bot…',
      rowsPerPage: 'Dòng/trang',
      prevPage: 'Trước',
      nextPage: 'Sau',
      page: 'Trang',
      colRank: '#',
      colBot: 'Bot',
      colMarket: 'Thị trường',
      colAlgo: 'Thuật toán',
      colStatus: 'Status',
      colInitCapital: 'Vốn ban đầu',
      colFinalValue: 'Giá trị cuối',
      noData: 'Chưa có dữ liệu',
    },
    // SimulationBot page
    simulationBot: {
      loadingBot: 'Đang tải chi tiết bot...',
      botNotFound: 'Không tìm thấy bot hoặc chưa có dữ liệu simulation.',
      backToLeaderboard: '← Quay lại Leaderboard',
      portfolioChart: 'Biểu đồ danh mục',
      chartValue: 'Giá trị',
      chartReturn: 'Return %',
      loadingChart: 'Đang tải biểu đồ...',
      onlyOneDay: 'Chỉ có 1 ngày dữ liệu',
      needMinTwoDays: 'Cần ít nhất 2 ngày để vẽ biểu đồ.',
      noChartData: 'Chưa có dữ liệu biểu đồ. Hãy chạy backtest trước.',
      botConfig: 'Cấu hình bot',
      tradingStrategy: 'chiến lược giao dịch',
      tradeHistory: 'Lịch sử giao dịch',
      loadingTrades: 'Đang tải giao dịch...',
      noTrades: 'Chưa có giao dịch nào. Hãy chạy backtest trước.',
      colSymbol: 'Mã',
      colType: 'Loại',
      colDate: 'Ngày',
      colPrice: 'Giá',
      colQuantity: 'Số lượng',
      colValue: 'Giá trị',
      colSignal: 'Tín hiệu',
      colCloseReason: 'Lý do đóng',
      pagePrev: '← Trước',
      pageNext: 'Tiếp →',
      page: 'Trang',
      trades: 'giao dịch',
      excellent: 'Xuất sắc',
      good: 'Tốt',
      low: 'Thấp',
      maxRisk: 'rủi ro tối đa',
      sharpeExcellent: 'Xuất sắc',
      sharpeGood: 'Tốt',
      sharpeLow: 'Thấp',
      transactions: 'giao dịch',
      runBacktestRequest: 'Đã gửi yêu cầu chạy backtest cho bot này',
      cannotSendRequest: 'Không thể gửi yêu cầu',
    },
    // Predictions page
    predictions: {
      totalPredictions: 'Tổng dự đoán',
      last30Days: '30 ngày qua',
      avgAccuracy: 'Độ chính xác TB',
      allModels: 'tất cả mô hình',
      confirmed: 'Đã xác nhận',
      withActualPrice: 'có giá thực tế',
      avgError: 'Sai số TB',
      errorDiff: '|dự đoán − thực tế|',
      algoPerformance: 'Hiệu suất thuật toán',
      accuracyLabel: 'độ chính xác',
      accuracyTrend: 'Xu hướng độ chính xác',
      recentSessions: 'phiên gần nhất',
      noTrainingData: 'Chưa có dữ liệu huấn luyện',
      noTrendData: 'Chưa có dữ liệu xu hướng',
      allPredictions: 'Tất cả dự đoán',
      noConfirmedPred: 'Chưa có dự đoán đã xác nhận',
      colSymbol: 'Mã',
      colAlgo: 'Thuật toán',
      colPredPrice: 'Giá dự đoán',
      colActualPrice: 'Giá thực tế',
      colPredDelta: 'Δ dự đoán',
      colActualDelta: 'Δ thực tế',
      colConfidence: 'Độ tin cậy',
      colAccuracy: 'Độ chính xác',
      colStatus: 'Trạng thái',
      statusHit: 'Chính xác',
      statusMiss: 'Sai lệch',
      statusNear: 'Gần đúng',
      compareChart: 'Dự đoán vs Thực tế',
      noCompareData: 'Chưa có dữ liệu so sánh',
      loadingCompare: 'Đang tải...',
      errorDist: 'Phân bố sai số',
      errorDistSub: 'độ tin cậy × |sai số|',
      noAnalysisData: 'Chưa có dữ liệu phân tích',
      confidenceAxis: 'Độ tin cậy (%)',
      top5Accurate: 'Top 5 chính xác nhất',
      top5Inaccurate: 'Top 5 sai lệch nhất',
      noData: 'Chưa có dữ liệu',
      predLabel: 'DĐ',
      actualLabel: 'TT',
      errorLabel: 'sai số',
    },
    // Training page
    training: {
      modelsTrained: 'Mô hình đã huấn luyện',
      bestAccuracy: 'Độ chính xác tốt nhất',
      totalData: 'Tổng dữ liệu',
      stocksTracked: 'cổ phiếu đang theo dõi',
      lastTrained: 'Lần huấn luyện cuối',
      automatic: 'tự động',
      runningTask: 'Tác vụ đang chạy',
      progress: 'Tiến độ',
      retrainAll: 'Huấn luyện lại tất cả',
      stop: 'Dừng',
      noRunningTask: 'Không có tác vụ nào đang chạy.',
      trainNow: 'Huấn luyện ngay',
      trainingQueue: 'Hàng đợi huấn luyện',
      tasks: 'tác vụ',
      noQueuedTasks: 'Không có tác vụ trong hàng đợi',
      statusRunning: 'Đang chạy',
      statusDone: 'Hoàn tất',
      statusWaiting: 'Chờ',
      models: 'Mô hình',
      noModelData: 'Chưa có dữ liệu mô hình. Bấm huấn luyện để bắt đầu.',
      trained: 'Đã huấn luyện',
      accuracyDelta: 'Δ độ chính xác',
      updated: 'Cập nhật',
      trainingLog: 'Nhật ký huấn luyện',
      realtime: 'thời gian thực',
      noLogs: 'Chưa có nhật ký huấn luyện.',
      trainRequest: 'Đã gửi yêu cầu huấn luyện',
      trainFail: 'Chưa gọi được API huấn luyện',
    },
    // MarketPredictions page
    marketPredictions: {
      allStatuses: 'Tất cả trạng thái',
      statusPending: 'Đang chờ',
      statusConfirmed: 'Đã xác nhận',
      records: 'bản ghi',
      searchPlaceholder: 'Tìm kiếm...',
      colSource: 'Nguồn',
      colProduct: 'Sản phẩm',
      colCoin: 'Coin',
      colSymbol: 'Mã',
      colPredPrice: 'Giá dự đoán',
      colActualPrice: 'Giá thực tế',
      colDelta: 'Δ dự đoán',
      colConfidence: 'Độ tin cậy',
      colAccuracy: 'Độ chính xác',
      colStatus: 'Trạng thái',
      colPredDate: 'Ngày dự đoán',
      badgePending: 'Đang chờ',
      badgeAccurate: 'Chính xác',
      badgeNear: 'Gần đúng',
      badgeDeviated: 'Sai lệch',
      noPredData: 'Chưa có dữ liệu dự đoán',
      prevPage: 'Trước',
      nextPage: 'Tiếp',
      cannotLoad: 'Không thể tải dữ liệu',
      unknownError: 'Lỗi không xác định',
    },
    // MarketTraining page
    marketTraining: {
      allAlgos: 'Tất cả thuật toán',
      sessions: 'phiên',
      colSession: 'Session ID',
      colAlgo: 'Thuật toán',
      colTotal: 'Tổng',
      colSuccess: 'Thành công',
      colError: 'Lỗi',
      colAccuracy: 'Độ chính xác',
      colDuration: 'Thời gian',
      colStarted: 'Bắt đầu',
      colCompleted: 'Hoàn thành',
      noTrainingHistory: 'Chưa có lịch sử huấn luyện',
      cannotLoad: 'Không thể tải dữ liệu',
      unknownError: 'Lỗi không xác định',
      prevPage: 'Trước',
      nextPage: 'Tiếp',
    },
    // MarketDetail page
    marketDetail: {
      algoComparison: 'So sánh thuật toán',
      loading: 'Đang tải...',
      noCompareData: 'Chưa có dữ liệu so sánh cho',
      cannotLoad: 'Không thể tải dữ liệu',
      unknownError: 'Lỗi không xác định',
      periodOptions: {
        d30: '30 ngày',
        d60: '60 ngày',
        d90: '90 ngày',
      },
    },
    // Guide page
    guide: {
      tableOfContents: 'Mục lục',
      sections: {
        start: 'Bắt đầu',
        markets: 'Thị trường',
        predictions: 'Dự đoán',
        training: 'Huấn luyện',
        simulation: 'Simulation',
        monitoring: 'Giám sát',
        api: 'API & Trigger',
      },
    },
    // Settings page
    settings: {
      accountInfo: 'Thông tin tài khoản',
      changePassword: 'Đổi mật khẩu',
      currentPassword: 'Mật khẩu hiện tại',
      newPassword: 'Mật khẩu mới',
      confirmNewPassword: 'Xác nhận mật khẩu mới',
      minChars: 'Tối thiểu 6 ký tự',
      updatePassword: 'Cập nhật mật khẩu',
      updating: 'Đang cập nhật...',
      passwordMismatch: 'Mật khẩu mới không khớp',
      passwordTooShort: 'Mật khẩu mới phải có ít nhất 6 ký tự',
      passwordSuccess: 'Đổi mật khẩu thành công',
      passwordFail: 'Đổi mật khẩu thất bại',
      cronSchedules: 'Lịch tác vụ tự động',
      loadingSchedules: 'Đang tải…',
      noScheduleData: 'Chưa có dữ liệu lịch tác vụ',
      cannotLoadSchedules: 'Không thể tải lịch tác vụ',
      colTask: 'Tác vụ',
      colSchedule: 'Lịch',
      colStatus: 'Trạng thái',
      save: 'Lưu',
      saving: '...',
      cancel: 'Huỷ',
      enabled: 'Bật',
      disabled: 'Tắt',
      manualTriggers: 'Thao tác thủ công',
      manualDesc: 'Chạy pipeline ngay lập tức: crawl dữ liệu mới → dự đoán → cập nhật bot trading',
      trainAll: 'Huấn luyện tất cả',
      reconcile: 'Reconcile',
      running: 'Đang chạy…',
      done: 'Hoàn thành',
      everyHour: 'Mỗi giờ',
      everyNHours: 'Mỗi N giờ',
      daily: 'Hàng ngày lúc',
      weekly: 'Hàng tuần vào',
      everyLabel: 'Mỗi',
      hourMinute: 'giờ : phút',
      hours: 'giờ',
      pipelineCrawl: 'Crawl',
      pipelinePredict: 'Dự đoán',
      pipelineBotTrading: 'Bot trading',
    },
    // Users page
    users: {
      createUser: 'Tạo người dùng mới',
      username: 'Tên đăng nhập',
      password: 'Mật khẩu',
      role: 'Vai trò',
      fullName: 'Họ và tên',
      email: 'Email',
      phone: 'Số điện thoại',
      createBtn: 'Tạo người dùng',
      creating: 'Đang tạo...',
      createFail: 'Tạo người dùng thất bại',
      userList: 'Danh sách người dùng',
      people: 'người dùng',
      loading: 'Đang tải...',
      cannotLoad: 'Không thể tải danh sách người dùng',
      noUsers: 'Chưa có người dùng nào.',
      colId: 'ID',
      colUsername: 'Username',
      colFullName: 'Họ và tên',
      colEmail: 'Email',
      colPhone: 'Điện thoại',
      colRole: 'Vai trò',
      colCreatedAt: 'Ngày tạo',
      deleteConfirm: 'Xóa người dùng',
      resetPassword: 'Đặt lại mật khẩu',
      resetPasswordFor: 'Đặt lại mật khẩu cho',
      newPassword: 'Mật khẩu mới',
      newPasswordPlaceholder: 'Tối thiểu 6 ký tự',
      resetBtn: 'Đặt lại',
      resetting: 'Đang đặt lại...',
      resetSuccess: 'Đặt lại mật khẩu thành công',
      resetFail: 'Đặt lại mật khẩu thất bại',
      passwordTooShort: 'Mật khẩu mới phải có ít nhất 6 ký tự',
      cancel: 'Hủy',
      optional: 'tùy chọn',
      editUser: 'Chỉnh sửa người dùng',
      editUserFor: 'Chỉnh sửa',
      save: 'Lưu',
      saving: 'Đang lưu...',
      updateSuccess: 'Cập nhật người dùng thành công',
      updateFail: 'Cập nhật người dùng thất bại',
    },
    // Market Groups page
    marketGroups: {
      createGroup: 'Tạo group mới',
      groupName: 'Tên group',
      groupDesc: 'Mô tả',
      groupDescPlaceholder: 'Mô tả (tùy chọn)',
      createBtn: 'Tạo',
      creating: 'Đang tạo...',
      noGroups: 'Chưa có group nào.',
      manage: 'Quản lý',
      collapse: 'Thu gọn',
      edit: 'Sửa',
      delete: 'Xóa',
      save: 'Lưu',
      cancel: 'Hủy',
      deleteConfirm: 'Xóa group này?',
      marketsSection: 'Markets',
      saveMarkets: 'Lưu markets',
      addUser: 'Thêm user',
      selectUser: '-- Chọn user --',
      addBtn: 'Thêm',
      usersInGroup: 'Users trong group',
      removeUser: 'Xóa',
      noUsers: 'Chưa có user.',
      loading: 'Đang tải...',
      groupCount: 'nhóm',
      optional: 'tùy chọn',
    },
    // Commands page
    commands: {
      pageTitle: 'Lệnh CLI',
      createCommand: 'Tạo lệnh',
      editCommand: 'Sửa lệnh',
      commandName: 'Tên lệnh',
      description: 'Mô tả',
      descriptionPlaceholder: 'Mô tả ngắn về lệnh này',
      optional: 'tùy chọn',
      handler: 'Handler',
      selectHandler: 'Chọn handler',
      argsSection: 'Tham số',
      noArgs: 'Handler này không có tham số',
      selectRequired: 'Bắt buộc',
      selectOptional: 'Tùy chọn',
      enabled: 'Đang bật',
      disabled: 'Đã tắt',
      save: 'Lưu',
      saving: 'Đang lưu...',
      cancel: 'Hủy',
      edit: 'Sửa',
      delete: 'Xóa',
      deleteConfirm: 'Xóa lệnh',
      deleteFail: 'Xóa lệnh thất bại',
      saveFail: 'Lưu lệnh thất bại',
      loading: 'Đang tải...',
      cannotLoad: 'Không thể tải danh sách lệnh',
      noCommands: 'Chưa có lệnh nào.',
      commandCount: 'lệnh',
      colName: 'Tên',
      colHandler: 'Handler',
      colArgs: 'Tham số',
      colStatus: 'Trạng thái',
      nameRequired: 'Vui lòng nhập tên lệnh',
      handlerRequired: 'Vui lòng chọn handler',
      argRequired: 'Tham số bắt buộc thiếu giá trị',
    },
    // CommandGroups page
    commandGroups: {
      createGroup: 'Tạo nhóm lệnh mới',
      groupName: 'Tên nhóm',
      groupDesc: 'Mô tả',
      groupDescPlaceholder: 'Mô tả (tùy chọn)',
      createBtn: 'Tạo',
      createFail: 'Tạo nhóm thất bại',
      noGroups: 'Chưa có nhóm lệnh nào.',
      manage: 'Quản lý',
      collapse: 'Thu gọn',
      edit: 'Sửa',
      delete: 'Xóa',
      save: 'Lưu',
      cancel: 'Hủy',
      saveFail: 'Lưu nhóm thất bại',
      deleteFail: 'Xóa nhóm thất bại',
      deleteConfirm: 'Xóa nhóm lệnh này?',
      commandsSection: 'Lệnh trong nhóm',
      commandsLabel: 'lệnh',
      saveCommands: 'Lưu danh sách lệnh',
      saveCommandsFail: 'Lưu danh sách lệnh thất bại',
      noCommandsAvailable: 'Chưa có lệnh nào. Hãy tạo lệnh trước.',
      disabledLabel: 'Tắt',
      addUser: 'Thêm người dùng',
      selectUser: '-- Chọn người dùng --',
      addBtn: 'Thêm',
      addUserFail: 'Thêm người dùng thất bại',
      usersInGroup: 'Người dùng trong nhóm',
      removeUser: 'Xóa',
      noUsers: 'Chưa có người dùng.',
      loading: 'Đang tải...',
      cannotLoad: 'Không thể tải danh sách nhóm lệnh',
      groupCount: 'nhóm',
      optional: 'tùy chọn',
    },
    // Monitoring page
    monitoring: {
      pageTitle: 'Giám sát dữ liệu',
      generatedAt: 'Được tạo lúc',
      refresh: 'Làm mới',
      loading: 'Đang tải dữ liệu giám sát...',
      error: 'Không thể tải dữ liệu giám sát',
      loginRequired: 'Vui lòng đăng nhập để xem trang này.',
      // Market card
      crawlSection: 'Thu thập dữ liệu',
      lastCrawl: 'Lần crawl cuối',
      staleness: 'Cũ',
      never: 'Chưa crawl',
      dailyToday: 'Bản ghi hôm nay',
      intradayToday: 'Intraday hôm nay',
      staleWarning: 'Dữ liệu cũ',
      marketClosed: 'Đóng cửa',
      predSection: 'Dự đoán',
      lastPredict: 'Dự đoán cuối',
      todayTotal: 'Tổng hôm nay',
      expectedAlgos: 'Thuật toán kỳ vọng',
      missingAlgos: 'Thiếu thuật toán',
      noMissing: 'Đủ thuật toán',
      algoTable: 'Chi tiết thuật toán',
      colAlgo: 'Thuật toán',
      colTodayCount: 'Hôm nay',
      colDirAccuracy: 'Độ CX hướng',
      colReconciled: 'Reconciled',
      colCorrect: 'Đúng',
      noAlgoData: 'Chưa có dữ liệu thuật toán',
      // Bots
      botsSummary: 'Tổng hợp Bot Trading',
      totalBots: 'Tổng bots',
      activeBots: 'Bots đang hoạt động',
      byMarket: 'Theo thị trường',
      colMarket: 'Thị trường',
      colTrades: 'Giao dịch',
      colWins: 'Thắng',
      colLosses: 'Thua',
      colWinRate: 'Win Rate',
      colPnl: 'Total P&L',
      noBotsData: 'Chưa có dữ liệu bot',
      botsTable: 'Bảng Bot',
      colBotId: 'Bot ID',
      colAlgoBot: 'Thuật toán',
      colWLBE: 'W/L/BE',
      colReturnPct: 'Return %',
      colProfitFactor: 'Profit Factor',
      colOpenPos: 'Vị thế mở',
      colUnrealized: 'Lãi/lỗ tạm tính',
      returnPctNote: 'Tính từ snapshot danh mục cuối ngày (mark-to-market, gồm cả vị thế đang mở) — số thật, không bị thiên lệch như win_rate',
      noBotsTable: 'Chưa có dữ liệu bot nào',
      sortAsc: 'Tăng dần',
      sortDesc: 'Giảm dần',
      allMarkets: 'Tất cả thị trường',
      filterAlgo: 'Lọc thuật toán…',
      filterBotId: 'Tìm Bot ID…',
      clearFilters: 'Xóa lọc',
      rowsPerPage: 'Dòng/trang',
      prevPage: 'Trước',
      nextPage: 'Sau',
      page: 'Trang',
    },
  },

  en: {
    nav: {
      overview: 'Overview',
      markets: 'Markets',
      simulation: 'Simulation',
      support: 'Support',
      guide: 'Guide',
      settings: 'Settings',
      users: 'Users',
      leaderboard: 'Leaderboard',
      monitoring: 'Data Pipeline',
      marketGroups: 'Market Groups',
      commands: 'Commands',
      commandGroups: 'Command Groups',
      docs: 'Docs',
    },
    marketSubs: {
      overview: 'Overview',
      predictions: 'Predictions',
      training: 'Training',
    },
    markets: {
      gold: 'Gold',
    },
    mob: {
      overview: 'Overview',
      gold: 'Gold',
      guide: 'Guide',
    },
    topbar: {
      searchPlaceholder: 'Search symbol, sector...',
      notifications: 'Notifications',
      lightTheme: 'Light',
      darkTheme: 'Dark',
      logout: 'Logout',
      login: 'Login',
    },
    titles: {
      '/': ['Market Overview', 'DASHBOARD'],
      '/markets/gold': ['Gold', 'GOLD'],
      '/markets/gold/predictions': ['Gold Predictions', 'PREDICTIONS · GOLD'],
      '/markets/gold/training': ['Gold Training', 'TRAINING · GOLD'],
      '/markets/crypto': ['Cryptocurrency', 'CRYPTO'],
      '/markets/nasdaq100': ['NASDAQ 100', 'NASDAQ 100 · US EQUITIES'],
      '/markets/nasdaq100/predictions': ['NASDAQ Predictions', 'PREDICTIONS · NASDAQ'],
      '/markets/nasdaq100/training': ['NASDAQ Training', 'TRAINING · NASDAQ'],
      '/markets/sp500': ['S&P 500', 'S&P 500 · US EQUITIES'],
      '/markets/sp500/predictions': ['S&P 500 Predictions', 'PREDICTIONS · S&P 500'],
      '/markets/sp500/training': ['S&P 500 Training', 'TRAINING · S&P 500'],
      '/markets/crypto/predictions': ['Crypto Predictions', 'PREDICTIONS · CRYPTO'],
      '/markets/crypto/training': ['Crypto Training', 'TRAINING · CRYPTO'],
      '/simulation': ['Simulation Leaderboard', 'SIMULATION · LEADERBOARD'],
      '/guide': ['User Guide', 'USER GUIDE'],
      '/settings': ['Settings', 'SETTINGS'],
      '/admin/users': ['User Management', 'ADMIN · USERS'],
      '/admin/market-groups': ['Market Groups', 'ADMIN · MARKET GROUPS'],
      '/gold': ['Gold Price', 'GOLD'],
      '/crypto': ['Cryptocurrency', 'CRYPTO'],
      '/nasdaq': ['NASDAQ 100', 'NASDAQ 100 · US EQUITIES'],
      '/monitoring': ['Data Pipeline', 'DATA PIPELINE · MONITORING'],
      '/admin/commands': ['CLI Commands', 'ADMIN · COMMANDS'],
      '/admin/command-groups': ['CLI Command Groups', 'ADMIN · COMMAND GROUPS'],
      '/admin/docs': ['Technical Docs', 'ADMIN · DOCS'],
    },
    footer: {
      liveData: 'Live data',
      loadingData: 'Loading data…',
      sampleData: 'Sample data',
      dataSession: 'Data · latest session',
    },
    error: {
      cannotDisplay: 'Cannot display this page with current data.',
      retry: 'Try again',
    },
    tweaks: {
      interface: 'Interface',
      accentColor: 'Accent color',
      density: 'Density',
      fontSize: 'Font size',
      ticker: 'Ticker bar',
    },
    marketTabs: {
      overview: 'Overview',
      predictions: 'Predictions',
      detail: 'Detail',
      training: 'Training',
      session: 'Session',
    },
    marketClosed: {
      weekend: 'Market is closed (weekend). Crawling, predictions and bots are paused until the next trading session.',
      holiday: 'Market is closed (US holiday). Crawling, predictions and bots are paused until the next trading session.',
    },
    dateRange: {
      today: 'Today',
      d7: '7D',
      d30: '30D',
      d90: '90D',
      d180: '180D',
      d365: '1Yr',
    },
    common: {
      loading: 'Loading...',
      noData: 'No data available',
      noDataCollect: 'No data. Please collect data first.',
      collect: 'Collect',
      predict: 'Predict',
      refresh: 'Refresh',
      exportCsv: 'Export CSV',
      allAlgos: 'All algorithms',
      allSymbols: 'All symbols',
      all: 'All',
      symbol: 'Symbol',
      coin: 'Coin',
      current: 'Current',
      predicted: 'Predicted',
      actual: 'Actual',
      deviation: 'Deviation',
      accuracy: 'Acc.',
      confidence: 'Confidence',
      date: 'Date',
      status: 'Status',
      noDataChart: 'No chart data. Please collect data first.',
      loadingChart: 'Loading chart...',
      noCompareData: 'No prediction comparison data',
      predVsActual: 'Prediction vs Actual',
      latestPredResults: 'Latest prediction results',
      noConfirmedResults: 'No confirmed results yet. Results will appear after predictions are reconciled with actual prices.',
      noPredictions: 'No predictions yet. Please run predictions first.',
      noDataForItem: 'No data for this item.',
      source: 'Source',
      product: 'Product',
      chartType: { line: 'Line', candle: 'Candle' },
    },
    dashboard: {
      liquidity: 'Liquidity',
      stocksMatched: 'shares matched',
      ensAccuracy: 'ENS Accuracy',
      ensModel: 'ensemble model',
      indexChart: 'Index Chart',
      sessionToday: "Today's session · close",
      noIndexData: 'No index data available',
      marketBreadth: 'Market Breadth',
      watchlist: 'Watchlist',
      noWatchData: 'No data. Loading...',
      colSymbol: 'Symbol',
      colPrice: 'Price',
      colChangePct: '±%',
      colVolume: 'Vol (M)',
      col7Sessions: '7 sessions',
      mlModelStatus: 'ML Model Status',
      algorithms: 'algorithms',
      noTrainingData: 'No training data',
      latestPredictions: 'Latest Predictions',
      noPredData: 'No predictions. Loading...',
      colCurrentPrice: 'Current Price',
      colPredPrice: 'Predicted Price',
      colExpectedDelta: 'Expected Δ',
      colAlgo: 'Algorithm',
      colConfidence: 'Confidence',
      colTargetSession: 'Target Session',
      algoAccuracy: 'Algorithm Accuracy Comparison',
      last30Sessions: '30 sessions',
      predByDay: 'Predictions by Day',
      last7Days: '7 days',
      noChartData: 'No data',
      predCount: 'predictions',
      topSimBots: 'Top Simulation Bots',
      topBotsDesc: 'Top 3 highest-performing bots',
      viewAll: 'View all →',
      noSimData: 'No simulation data.',
      runBacktest: 'Run backtest',
      up: 'Up',
      down: 'Down',
      ref: 'Flat',
      rising: 'advancing',
      leadingSectors: 'Leading Sectors',
      noMarketData: 'No market data',
      totalPredictions: 'Total Predictions',
      allTime: 'all time',
      predsToday: 'Predictions Today',
      acrossMarkets: 'across all markets',
      activeBots: 'Active Bots',
      inSimulation: 'running simulation',
      marketStatus: 'Pipeline Status',
      marketStatusDesc: 'Data freshness & prediction activity',
      lastCrawl: 'Last crawl',
      predsCount: 'preds',
      loginForStatus: 'Log in to see system pipeline status',
      loadingStatus: 'Loading status...',
      marketOpen: 'Open',
      marketClosed: 'Closed',
      fresh: 'Fresh',
      stale: 'Stale',
      never: 'Never',
      missing: 'missing',
      dirAccuracy: 'Direction Prediction Accuracy',
      dirAccuracySub: 'avg across markets · from reconcile history',
      dirAccuracyChart: 'Direction Accuracy Comparison',
      loginToSeeAccuracy: 'Log in to see real direction accuracy from DB',
      noReconcileData: 'No reconcile data yet',
      ensDir: 'ENS Direction Acc',
      ensDirSub: 'avg across markets',
    },
    crypto: {
      priceChart: 'Crypto Price Chart',
      noAccess: 'You do not have access to the Crypto market.',
      loadingData: 'Loading crypto data...',
      noDataKpi: 'No data',
      tomorrowPred: "Tomorrow's Crypto Predictions",
      noPredForCoin: 'No data for this coin.',
      marketInfo: 'Crypto Market Info',
      collectRequest: 'Crypto collection request sent',
      collectFail: 'Cannot send collection request',
      predictRequest: 'Crypto prediction request sent',
      predictFail: 'Cannot send prediction request',
      noChartData: 'No chart data. Please collect data first.',
      noConfirmedForCoin: 'No data for this coin.',
      noCompareData: 'No prediction comparison data for',
    },
    gold: {
      priceChart: 'Gold Price Chart',
      noAccess: 'You do not have access to the Gold market.',
      loadingData: 'Loading gold data...',
      loading: 'loading...',
      sellPrice: 'sell price',
      providerComparison: 'Provider Price Comparison',
      colProvider: 'Provider',
      colType: 'Type',
      colRegion: 'Region',
      colBuyPrice: 'Buy Price',
      colSellPrice: 'Sell Price',
      colSpread: 'Spread',
      tomorrowPred: "Tomorrow's Gold Predictions",
      runPrediction: 'Run Prediction',
      historicalBacktest: 'Historical Backtest',
      noPredForType: 'No data for this type.',
      noGoldPred: 'No gold predictions yet',
      collectRequest: 'Gold collection request sent',
      collectFail: 'Cannot call collection API',
      predictRequest: 'Gold prediction request sent',
      predictFail: 'Cannot call prediction API',
      backtestRequest: 'Running gold backtest...',
      backtestFail: 'Cannot call backtest API',
      sjcDetail: 'SJC 1 Tael Detail',
      last10Days: 'Last 10 days',
      colDate: 'Date',
      colBuyVnd: 'Buy Price (VND)',
      colSellVnd: 'Sell Price (VND)',
      colSpreadLabel: 'Spread',
      noHistData: 'No historical data',
      noGoldHistory: 'No gold price history. Click collect to get data.',
      noConfirmedForType: 'No data for this type.',
      noCompareData: 'No prediction comparison data',
      subEnsemble: 'BTMC SJC · Ensemble',
    },
    nasdaq: {
      priceChart: 'NASDAQ 100 Price Chart',
      noAccess: 'You do not have access to the NASDAQ market.',
      loadingData: 'Loading NASDAQ 100 data...',
      noDataKpi: 'No data',
      tomorrowPred: "Tomorrow's NASDAQ Predictions",
      priceTable: 'NASDAQ 100 Price Table',
      collectRequest: 'NASDAQ collection request sent',
      collectFail: 'Cannot send collection request',
      predictRequest: 'NASDAQ prediction request sent',
      predictFail: 'Cannot send prediction request',
      noChartData: 'No chart data. Please collect data first.',
      noDataForSymbol: 'No data for this symbol.',
      allAlgos: 'All algorithms',
    },
    sp500: {
      priceChart: 'S&P 500 Price Chart',
      noAccess: 'You do not have access to the S&P 500 market.',
      loadingData: 'Loading S&P 500 data...',
      noDataKpi: 'No data',
      tomorrowPred: "Tomorrow's S&P 500 Predictions",
      priceTable: 'S&P 500 Price Table',
      collectRequest: 'S&P 500 collection request sent',
      collectFail: 'Cannot send collection request',
      predictRequest: 'S&P 500 prediction request sent',
      predictFail: 'Cannot send prediction request',
      noChartData: 'No chart data. Please collect data first.',
      noDataForSymbol: 'No data for this symbol.',
      noConfirmed: 'No confirmed results yet. Results will appear after predictions are reconciled with actual prices.',
    },
    simulation: {
      totalBots: 'Total Bots',
      monitored: 'being tracked',
      bestMarket: 'Best Market',
      byAvgReturn: 'by avg return',
      bestAlgo: 'Best Algorithm',
      avgPerformance: 'avg performance',
      avgReturn: 'Avg Return',
      loadingData: 'Loading simulation data...',
      cannotLoad: 'Cannot load data. Backend has no simulation data yet.',
      topReturnBots: 'Top Bots by Return %',
      highest: 'highest',
      marketDist: 'Market Distribution',
      byBotCount: 'by bot count',
      leaderboard: 'Simulation Leaderboard',
      bots: 'bots',
      allAlgos: 'All algorithms',
      runAllBacktest: 'All-bots backtest request sent',
      cannotSendRequest: 'Cannot send request',
      noSimData: 'No simulation data. Please run backtest first.',
      runNow: 'Run backtest now',
      searchBot: 'Search bot…',
      rowsPerPage: 'Rows/page',
      prevPage: 'Prev',
      nextPage: 'Next',
      page: 'Page',
      colRank: '#',
      colBot: 'Bot',
      colMarket: 'Market',
      colAlgo: 'Algorithm',
      colStatus: 'Status',
      colInitCapital: 'Initial Capital',
      colFinalValue: 'Final Value',
      noData: 'No data',
    },
    simulationBot: {
      loadingBot: 'Loading bot details...',
      botNotFound: 'Bot not found or no simulation data available.',
      backToLeaderboard: '← Back to Leaderboard',
      portfolioChart: 'Portfolio Chart',
      chartValue: 'Value',
      chartReturn: 'Return %',
      loadingChart: 'Loading chart...',
      onlyOneDay: 'Only 1 day of data',
      needMinTwoDays: 'Need at least 2 days to draw chart.',
      noChartData: 'No chart data. Please run backtest first.',
      botConfig: 'Bot Configuration',
      tradingStrategy: 'trading strategy',
      tradeHistory: 'Trade History',
      loadingTrades: 'Loading trades...',
      noTrades: 'No trades yet. Please run backtest first.',
      colSymbol: 'Symbol',
      colType: 'Type',
      colDate: 'Date',
      colPrice: 'Price',
      colQuantity: 'Qty',
      colValue: 'Value',
      colSignal: 'Signal',
      colCloseReason: 'Close Reason',
      pagePrev: '← Prev',
      pageNext: 'Next →',
      page: 'Page',
      trades: 'trades',
      excellent: 'Excellent',
      good: 'Good',
      low: 'Low',
      maxRisk: 'max risk',
      sharpeExcellent: 'Excellent',
      sharpeGood: 'Good',
      sharpeLow: 'Low',
      transactions: 'trades',
      runBacktestRequest: 'Backtest request sent for this bot',
      cannotSendRequest: 'Cannot send request',
    },
    predictions: {
      totalPredictions: 'Total Predictions',
      last30Days: 'last 30 days',
      avgAccuracy: 'Avg Accuracy',
      allModels: 'all models',
      confirmed: 'Confirmed',
      withActualPrice: 'with actual price',
      avgError: 'Avg Error',
      errorDiff: '|predicted − actual|',
      algoPerformance: 'Algorithm Performance',
      accuracyLabel: 'accuracy',
      accuracyTrend: 'Accuracy Trend',
      recentSessions: 'recent sessions',
      noTrainingData: 'No training data',
      noTrendData: 'No trend data',
      allPredictions: 'All Predictions',
      noConfirmedPred: 'No confirmed predictions',
      colSymbol: 'Symbol',
      colAlgo: 'Algorithm',
      colPredPrice: 'Predicted Price',
      colActualPrice: 'Actual Price',
      colPredDelta: 'Pred Δ',
      colActualDelta: 'Actual Δ',
      colConfidence: 'Confidence',
      colAccuracy: 'Accuracy',
      colStatus: 'Status',
      statusHit: 'Accurate',
      statusMiss: 'Missed',
      statusNear: 'Near',
      compareChart: 'Prediction vs Actual',
      noCompareData: 'No comparison data',
      loadingCompare: 'Loading...',
      errorDist: 'Error Distribution',
      errorDistSub: 'confidence × |error|',
      noAnalysisData: 'No analysis data',
      confidenceAxis: 'Confidence (%)',
      top5Accurate: 'Top 5 Most Accurate',
      top5Inaccurate: 'Top 5 Least Accurate',
      noData: 'No data',
      predLabel: 'Pred',
      actualLabel: 'Act',
      errorLabel: 'error',
    },
    training: {
      modelsTrained: 'Models Trained',
      bestAccuracy: 'Best Accuracy',
      totalData: 'Total Data',
      stocksTracked: 'stocks tracked',
      lastTrained: 'Last Trained',
      automatic: 'automatic',
      runningTask: 'Running Task',
      progress: 'Progress',
      retrainAll: 'Retrain All',
      stop: 'Stop',
      noRunningTask: 'No tasks running.',
      trainNow: 'Train Now',
      trainingQueue: 'Training Queue',
      tasks: 'tasks',
      noQueuedTasks: 'No queued tasks',
      statusRunning: 'Running',
      statusDone: 'Done',
      statusWaiting: 'Waiting',
      models: 'Models',
      noModelData: 'No model data. Click train to start.',
      trained: 'Trained',
      accuracyDelta: 'Δ Accuracy',
      updated: 'Updated',
      trainingLog: 'Training Log',
      realtime: 'real-time',
      noLogs: 'No training logs.',
      trainRequest: 'Training request sent',
      trainFail: 'Cannot call training API',
    },
    marketPredictions: {
      allStatuses: 'All statuses',
      statusPending: 'Pending',
      statusConfirmed: 'Confirmed',
      records: 'records',
      searchPlaceholder: 'Search...',
      colSource: 'Source',
      colProduct: 'Product',
      colCoin: 'Coin',
      colSymbol: 'Symbol',
      colPredPrice: 'Predicted Price',
      colActualPrice: 'Actual Price',
      colDelta: 'Pred Δ',
      colConfidence: 'Confidence',
      colAccuracy: 'Accuracy',
      colStatus: 'Status',
      colPredDate: 'Prediction Date',
      badgePending: 'Pending',
      badgeAccurate: 'Accurate',
      badgeNear: 'Near',
      badgeDeviated: 'Deviated',
      noPredData: 'No prediction data',
      prevPage: 'Prev',
      nextPage: 'Next',
      cannotLoad: 'Cannot load data',
      unknownError: 'Unknown error',
    },
    marketTraining: {
      allAlgos: 'All algorithms',
      sessions: 'sessions',
      colSession: 'Session ID',
      colAlgo: 'Algorithm',
      colTotal: 'Total',
      colSuccess: 'Success',
      colError: 'Error',
      colAccuracy: 'Accuracy',
      colDuration: 'Duration',
      colStarted: 'Started',
      colCompleted: 'Completed',
      noTrainingHistory: 'No training history',
      cannotLoad: 'Cannot load data',
      unknownError: 'Unknown error',
      prevPage: 'Prev',
      nextPage: 'Next',
    },
    marketDetail: {
      algoComparison: 'Algorithm Comparison',
      loading: 'Loading...',
      noCompareData: 'No comparison data for',
      cannotLoad: 'Cannot load data',
      unknownError: 'Unknown error',
      periodOptions: {
        d30: '30 days',
        d60: '60 days',
        d90: '90 days',
      },
    },
    guide: {
      tableOfContents: 'Table of Contents',
      sections: {
        start: 'Getting Started',
        markets: 'Markets',
        predictions: 'Predictions',
        training: 'Training',
        simulation: 'Simulation',
        monitoring: 'Monitoring',
        api: 'API & Trigger',
      },
    },
    settings: {
      accountInfo: 'Account Info',
      changePassword: 'Change Password',
      currentPassword: 'Current Password',
      newPassword: 'New Password',
      confirmNewPassword: 'Confirm New Password',
      minChars: 'Minimum 6 characters',
      updatePassword: 'Update Password',
      updating: 'Updating...',
      passwordMismatch: 'New passwords do not match',
      passwordTooShort: 'New password must be at least 6 characters',
      passwordSuccess: 'Password changed successfully',
      passwordFail: 'Failed to change password',
      cronSchedules: 'Automated Task Schedules',
      loadingSchedules: 'Loading…',
      noScheduleData: 'No schedule data',
      cannotLoadSchedules: 'Cannot load schedules',
      colTask: 'Task',
      colSchedule: 'Schedule',
      colStatus: 'Status',
      save: 'Save',
      saving: '...',
      cancel: 'Cancel',
      enabled: 'On',
      disabled: 'Off',
      manualTriggers: 'Manual Triggers',
      manualDesc: 'Run pipeline immediately: crawl new data → predict → update trading bots',
      trainAll: 'Train All',
      reconcile: 'Reconcile',
      running: 'Running…',
      done: 'Done',
      everyHour: 'Every hour',
      everyNHours: 'Every N hours',
      daily: 'Daily at',
      weekly: 'Weekly on',
      everyLabel: 'Every',
      hourMinute: 'hour : minute',
      hours: 'hours',
      pipelineCrawl: 'Crawl',
      pipelinePredict: 'Predict',
      pipelineBotTrading: 'Bot trading',
    },
    users: {
      createUser: 'Create New User',
      username: 'Username',
      password: 'Password',
      role: 'Role',
      fullName: 'Full name',
      email: 'Email',
      phone: 'Phone',
      createBtn: 'Create User',
      creating: 'Creating...',
      createFail: 'Failed to create user',
      userList: 'User List',
      people: 'users',
      loading: 'Loading...',
      cannotLoad: 'Cannot load user list',
      noUsers: 'No users found.',
      colId: 'ID',
      colUsername: 'Username',
      colFullName: 'Full name',
      colEmail: 'Email',
      colPhone: 'Phone',
      colRole: 'Role',
      colCreatedAt: 'Created At',
      deleteConfirm: 'Delete user',
      resetPassword: 'Reset password',
      resetPasswordFor: 'Reset password for',
      newPassword: 'New password',
      newPasswordPlaceholder: 'Minimum 6 characters',
      resetBtn: 'Reset',
      resetting: 'Resetting...',
      resetSuccess: 'Password reset successfully',
      resetFail: 'Failed to reset password',
      passwordTooShort: 'New password must be at least 6 characters',
      cancel: 'Cancel',
      optional: 'optional',
      editUser: 'Edit user',
      editUserFor: 'Edit',
      save: 'Save',
      saving: 'Saving...',
      updateSuccess: 'User updated successfully',
      updateFail: 'Failed to update user',
    },
    // Market Groups page
    marketGroups: {
      createGroup: 'Create New Group',
      groupName: 'Group name',
      groupDesc: 'Description',
      groupDescPlaceholder: 'Description (optional)',
      createBtn: 'Create',
      creating: 'Creating...',
      noGroups: 'No groups yet.',
      manage: 'Manage',
      collapse: 'Collapse',
      edit: 'Edit',
      delete: 'Delete',
      save: 'Save',
      cancel: 'Cancel',
      deleteConfirm: 'Delete this group?',
      marketsSection: 'Markets',
      saveMarkets: 'Save markets',
      addUser: 'Add user',
      selectUser: '-- Select user --',
      addBtn: 'Add',
      usersInGroup: 'Users in group',
      removeUser: 'Remove',
      noUsers: 'No users.',
      loading: 'Loading...',
      groupCount: 'groups',
      optional: 'optional',
    },
    // Commands page
    commands: {
      pageTitle: 'CLI Commands',
      createCommand: 'Create Command',
      editCommand: 'Edit Command',
      commandName: 'Command name',
      description: 'Description',
      descriptionPlaceholder: 'Short description of this command',
      optional: 'optional',
      handler: 'Handler',
      selectHandler: 'Select handler',
      argsSection: 'Arguments',
      noArgs: 'This handler has no arguments',
      selectRequired: 'Required',
      selectOptional: 'Optional',
      enabled: 'Enabled',
      disabled: 'Disabled',
      save: 'Save',
      saving: 'Saving...',
      cancel: 'Cancel',
      edit: 'Edit',
      delete: 'Delete',
      deleteConfirm: 'Delete command',
      deleteFail: 'Failed to delete command',
      saveFail: 'Failed to save command',
      loading: 'Loading...',
      cannotLoad: 'Cannot load command list',
      noCommands: 'No commands yet.',
      commandCount: 'commands',
      colName: 'Name',
      colHandler: 'Handler',
      colArgs: 'Args',
      colStatus: 'Status',
      nameRequired: 'Command name is required',
      handlerRequired: 'Please select a handler',
      argRequired: 'Required argument missing value',
    },
    // CommandGroups page
    commandGroups: {
      createGroup: 'Create New Command Group',
      groupName: 'Group name',
      groupDesc: 'Description',
      groupDescPlaceholder: 'Description (optional)',
      createBtn: 'Create',
      createFail: 'Failed to create group',
      noGroups: 'No command groups yet.',
      manage: 'Manage',
      collapse: 'Collapse',
      edit: 'Edit',
      delete: 'Delete',
      save: 'Save',
      cancel: 'Cancel',
      saveFail: 'Failed to save group',
      deleteFail: 'Failed to delete group',
      deleteConfirm: 'Delete this command group?',
      commandsSection: 'Commands in group',
      commandsLabel: 'commands',
      saveCommands: 'Save commands',
      saveCommandsFail: 'Failed to save command list',
      noCommandsAvailable: 'No commands yet. Create commands first.',
      disabledLabel: 'Disabled',
      addUser: 'Add user',
      selectUser: '-- Select user --',
      addBtn: 'Add',
      addUserFail: 'Failed to add user',
      usersInGroup: 'Users in group',
      removeUser: 'Remove',
      noUsers: 'No users.',
      loading: 'Loading...',
      cannotLoad: 'Cannot load command groups',
      groupCount: 'groups',
      optional: 'optional',
    },
    // Monitoring page
    monitoring: {
      pageTitle: 'Data Pipeline',
      generatedAt: 'Generated at',
      refresh: 'Refresh',
      loading: 'Loading monitoring data...',
      error: 'Cannot load monitoring data',
      loginRequired: 'Please log in to view this page.',
      // Market card
      crawlSection: 'Data Crawl',
      lastCrawl: 'Last crawl',
      staleness: 'Age',
      never: 'Never crawled',
      dailyToday: "Today's records",
      intradayToday: "Today's intraday",
      staleWarning: 'Stale data',
      marketClosed: 'Market closed',
      predSection: 'Predictions',
      lastPredict: 'Last prediction',
      todayTotal: "Today's total",
      expectedAlgos: 'Expected algos',
      missingAlgos: 'Missing algorithms',
      noMissing: 'All algorithms present',
      algoTable: 'Algorithm detail',
      colAlgo: 'Algorithm',
      colTodayCount: 'Today',
      colDirAccuracy: 'Dir. Accuracy',
      colReconciled: 'Reconciled',
      colCorrect: 'Correct',
      noAlgoData: 'No algorithm data',
      // Bots
      botsSummary: 'Bot Trading Summary',
      totalBots: 'Total bots',
      activeBots: 'Active bots',
      byMarket: 'By market',
      colMarket: 'Market',
      colTrades: 'Trades',
      colWins: 'Wins',
      colLosses: 'Losses',
      colWinRate: 'Win Rate',
      colPnl: 'Total P&L',
      noBotsData: 'No bot data',
      botsTable: 'Bots Table',
      colBotId: 'Bot ID',
      colAlgoBot: 'Algorithm',
      colWLBE: 'W/L/BE',
      colReturnPct: 'Return %',
      colProfitFactor: 'Profit Factor',
      colOpenPos: 'Open Pos',
      colUnrealized: 'Unrealized P&L',
      returnPctNote: 'Derived from end-of-day portfolio snapshot (mark-to-market, includes open positions) — the honest metric, not survivorship-biased like win_rate',
      noBotsTable: 'No bots data',
      sortAsc: 'Ascending',
      sortDesc: 'Descending',
      allMarkets: 'All markets',
      filterAlgo: 'Filter algorithm…',
      filterBotId: 'Search Bot ID…',
      clearFilters: 'Clear filters',
      rowsPerPage: 'Rows/page',
      prevPage: 'Prev',
      nextPage: 'Next',
      page: 'Page',
    },
  },
};

// Structural interface — string values only, not literal types
export interface Translations {
  nav: {
    overview: string;
    markets: string;
    simulation: string;
    support: string;
    guide: string;
    settings: string;
    users: string;
    leaderboard: string;
    monitoring: string;
    marketGroups: string;
    commands: string;
    commandGroups: string;
    docs: string;
  };
  marketSubs: {
    overview: string;
    predictions: string;
    training: string;
  };
  markets: {
    gold: string;
  };
  mob: {
    overview: string;
    gold: string;
    guide: string;
  };
  topbar: {
    searchPlaceholder: string;
    notifications: string;
    lightTheme: string;
    darkTheme: string;
    logout: string;
    login: string;
  };
  titles: Record<string, [string, string]>;
  footer: {
    liveData: string;
    loadingData: string;
    sampleData: string;
    dataSession: string;
  };
  error: {
    cannotDisplay: string;
    retry: string;
  };
  tweaks: {
    interface: string;
    accentColor: string;
    density: string;
    fontSize: string;
    ticker: string;
  };
  marketTabs: {
    overview: string;
    predictions: string;
    detail: string;
    training: string;
    session: string;
  };
  marketClosed: {
    weekend: string;
    holiday: string;
  };
  dateRange: {
    today: string;
    d7: string;
    d30: string;
    d90: string;
    d180: string;
    d365: string;
  };
  common: {
    loading: string;
    noData: string;
    noDataCollect: string;
    collect: string;
    predict: string;
    refresh: string;
    exportCsv: string;
    allAlgos: string;
    allSymbols: string;
    all: string;
    symbol: string;
    coin: string;
    current: string;
    predicted: string;
    actual: string;
    deviation: string;
    accuracy: string;
    confidence: string;
    date: string;
    status: string;
    noDataChart: string;
    loadingChart: string;
    noCompareData: string;
    predVsActual: string;
    latestPredResults: string;
    noConfirmedResults: string;
    noPredictions: string;
    noDataForItem: string;
    source: string;
    product: string;
    chartType: { line: string; candle: string };
  };
  dashboard: {
    liquidity: string;
    stocksMatched: string;
    ensAccuracy: string;
    ensModel: string;
    indexChart: string;
    sessionToday: string;
    noIndexData: string;
    marketBreadth: string;
    watchlist: string;
    noWatchData: string;
    colSymbol: string;
    colPrice: string;
    colChangePct: string;
    colVolume: string;
    col7Sessions: string;
    mlModelStatus: string;
    algorithms: string;
    noTrainingData: string;
    latestPredictions: string;
    noPredData: string;
    colCurrentPrice: string;
    colPredPrice: string;
    colExpectedDelta: string;
    colAlgo: string;
    colConfidence: string;
    colTargetSession: string;
    algoAccuracy: string;
    last30Sessions: string;
    predByDay: string;
    last7Days: string;
    noChartData: string;
    predCount: string;
    topSimBots: string;
    topBotsDesc: string;
    viewAll: string;
    noSimData: string;
    runBacktest: string;
    up: string;
    down: string;
    ref: string;
    rising: string;
    leadingSectors: string;
    noMarketData: string;
    totalPredictions: string;
    allTime: string;
    predsToday: string;
    acrossMarkets: string;
    activeBots: string;
    inSimulation: string;
    marketStatus: string;
    marketStatusDesc: string;
    lastCrawl: string;
    predsCount: string;
    loginForStatus: string;
    loadingStatus: string;
    marketOpen: string;
    marketClosed: string;
    fresh: string;
    stale: string;
    never: string;
    missing: string;
    dirAccuracy: string;
    dirAccuracySub: string;
    dirAccuracyChart: string;
    loginToSeeAccuracy: string;
    noReconcileData: string;
    ensDir: string;
    ensDirSub: string;
  };
  crypto: {
    priceChart: string;
    noAccess: string;
    loadingData: string;
    noDataKpi: string;
    tomorrowPred: string;
    noPredForCoin: string;
    marketInfo: string;
    collectRequest: string;
    collectFail: string;
    predictRequest: string;
    predictFail: string;
    noChartData: string;
    noConfirmedForCoin: string;
    noCompareData: string;
  };
  gold: {
    priceChart: string;
    noAccess: string;
    loadingData: string;
    loading: string;
    sellPrice: string;
    providerComparison: string;
    colProvider: string;
    colType: string;
    colRegion: string;
    colBuyPrice: string;
    colSellPrice: string;
    colSpread: string;
    tomorrowPred: string;
    runPrediction: string;
    historicalBacktest: string;
    noPredForType: string;
    noGoldPred: string;
    collectRequest: string;
    collectFail: string;
    predictRequest: string;
    predictFail: string;
    backtestRequest: string;
    backtestFail: string;
    sjcDetail: string;
    last10Days: string;
    colDate: string;
    colBuyVnd: string;
    colSellVnd: string;
    colSpreadLabel: string;
    noHistData: string;
    noGoldHistory: string;
    noConfirmedForType: string;
    noCompareData: string;
    subEnsemble: string;
  };
  nasdaq: {
    priceChart: string;
    noAccess: string;
    loadingData: string;
    noDataKpi: string;
    tomorrowPred: string;
    priceTable: string;
    collectRequest: string;
    collectFail: string;
    predictRequest: string;
    predictFail: string;
    noChartData: string;
    noDataForSymbol: string;
    allAlgos: string;
  };
  sp500: {
    priceChart: string;
    noAccess: string;
    loadingData: string;
    noDataKpi: string;
    tomorrowPred: string;
    priceTable: string;
    collectRequest: string;
    collectFail: string;
    predictRequest: string;
    predictFail: string;
    noChartData: string;
    noDataForSymbol: string;
    noConfirmed: string;
  };
  simulation: {
    totalBots: string;
    monitored: string;
    bestMarket: string;
    byAvgReturn: string;
    bestAlgo: string;
    avgPerformance: string;
    avgReturn: string;
    loadingData: string;
    cannotLoad: string;
    topReturnBots: string;
    highest: string;
    marketDist: string;
    byBotCount: string;
    leaderboard: string;
    bots: string;
    allAlgos: string;
    runAllBacktest: string;
    cannotSendRequest: string;
    noSimData: string;
    runNow: string;
    searchBot: string;
    rowsPerPage: string;
    prevPage: string;
    nextPage: string;
    page: string;
    colRank: string;
    colBot: string;
    colMarket: string;
    colAlgo: string;
    colStatus: string;
    colInitCapital: string;
    colFinalValue: string;
    noData: string;
  };
  simulationBot: {
    loadingBot: string;
    botNotFound: string;
    backToLeaderboard: string;
    portfolioChart: string;
    chartValue: string;
    chartReturn: string;
    loadingChart: string;
    onlyOneDay: string;
    needMinTwoDays: string;
    noChartData: string;
    botConfig: string;
    tradingStrategy: string;
    tradeHistory: string;
    loadingTrades: string;
    noTrades: string;
    colSymbol: string;
    colType: string;
    colDate: string;
    colPrice: string;
    colQuantity: string;
    colValue: string;
    colSignal: string;
    colCloseReason: string;
    pagePrev: string;
    pageNext: string;
    page: string;
    trades: string;
    excellent: string;
    good: string;
    low: string;
    maxRisk: string;
    sharpeExcellent: string;
    sharpeGood: string;
    sharpeLow: string;
    transactions: string;
    runBacktestRequest: string;
    cannotSendRequest: string;
  };
  predictions: {
    totalPredictions: string;
    last30Days: string;
    avgAccuracy: string;
    allModels: string;
    confirmed: string;
    withActualPrice: string;
    avgError: string;
    errorDiff: string;
    algoPerformance: string;
    accuracyLabel: string;
    accuracyTrend: string;
    recentSessions: string;
    noTrainingData: string;
    noTrendData: string;
    allPredictions: string;
    noConfirmedPred: string;
    colSymbol: string;
    colAlgo: string;
    colPredPrice: string;
    colActualPrice: string;
    colPredDelta: string;
    colActualDelta: string;
    colConfidence: string;
    colAccuracy: string;
    colStatus: string;
    statusHit: string;
    statusMiss: string;
    statusNear: string;
    compareChart: string;
    noCompareData: string;
    loadingCompare: string;
    errorDist: string;
    errorDistSub: string;
    noAnalysisData: string;
    confidenceAxis: string;
    top5Accurate: string;
    top5Inaccurate: string;
    noData: string;
    predLabel: string;
    actualLabel: string;
    errorLabel: string;
  };
  training: {
    modelsTrained: string;
    bestAccuracy: string;
    totalData: string;
    stocksTracked: string;
    lastTrained: string;
    automatic: string;
    runningTask: string;
    progress: string;
    retrainAll: string;
    stop: string;
    noRunningTask: string;
    trainNow: string;
    trainingQueue: string;
    tasks: string;
    noQueuedTasks: string;
    statusRunning: string;
    statusDone: string;
    statusWaiting: string;
    models: string;
    noModelData: string;
    trained: string;
    accuracyDelta: string;
    updated: string;
    trainingLog: string;
    realtime: string;
    noLogs: string;
    trainRequest: string;
    trainFail: string;
  };
  marketPredictions: {
    allStatuses: string;
    statusPending: string;
    statusConfirmed: string;
    records: string;
    searchPlaceholder: string;
    colSource: string;
    colProduct: string;
    colCoin: string;
    colSymbol: string;
    colPredPrice: string;
    colActualPrice: string;
    colDelta: string;
    colConfidence: string;
    colAccuracy: string;
    colStatus: string;
    colPredDate: string;
    badgePending: string;
    badgeAccurate: string;
    badgeNear: string;
    badgeDeviated: string;
    noPredData: string;
    prevPage: string;
    nextPage: string;
    cannotLoad: string;
    unknownError: string;
  };
  marketTraining: {
    allAlgos: string;
    sessions: string;
    colSession: string;
    colAlgo: string;
    colTotal: string;
    colSuccess: string;
    colError: string;
    colAccuracy: string;
    colDuration: string;
    colStarted: string;
    colCompleted: string;
    noTrainingHistory: string;
    cannotLoad: string;
    unknownError: string;
    prevPage: string;
    nextPage: string;
  };
  marketDetail: {
    algoComparison: string;
    loading: string;
    noCompareData: string;
    cannotLoad: string;
    unknownError: string;
    periodOptions: {
      d30: string;
      d60: string;
      d90: string;
    };
  };
  guide: {
    tableOfContents: string;
    sections: {
      start: string;
      markets: string;
      predictions: string;
      training: string;
      simulation: string;
      monitoring: string;
      api: string;
    };
  };
  settings: {
    accountInfo: string;
    changePassword: string;
    currentPassword: string;
    newPassword: string;
    confirmNewPassword: string;
    minChars: string;
    updatePassword: string;
    updating: string;
    passwordMismatch: string;
    passwordTooShort: string;
    passwordSuccess: string;
    passwordFail: string;
    cronSchedules: string;
    loadingSchedules: string;
    noScheduleData: string;
    cannotLoadSchedules: string;
    colTask: string;
    colSchedule: string;
    colStatus: string;
    save: string;
    saving: string;
    cancel: string;
    enabled: string;
    disabled: string;
    manualTriggers: string;
    manualDesc: string;
    trainAll: string;
    reconcile: string;
    running: string;
    done: string;
    everyHour: string;
    everyNHours: string;
    daily: string;
    weekly: string;
    everyLabel: string;
    hourMinute: string;
    hours: string;
    pipelineCrawl: string;
    pipelinePredict: string;
    pipelineBotTrading: string;
  };
  users: {
    createUser: string;
    username: string;
    password: string;
    role: string;
    fullName: string;
    email: string;
    phone: string;
    createBtn: string;
    creating: string;
    createFail: string;
    userList: string;
    people: string;
    loading: string;
    cannotLoad: string;
    noUsers: string;
    colId: string;
    colUsername: string;
    colFullName: string;
    colEmail: string;
    colPhone: string;
    colRole: string;
    colCreatedAt: string;
    deleteConfirm: string;
    resetPassword: string;
    resetPasswordFor: string;
    newPassword: string;
    newPasswordPlaceholder: string;
    resetBtn: string;
    resetting: string;
    resetSuccess: string;
    resetFail: string;
    passwordTooShort: string;
    cancel: string;
    optional: string;
    editUser: string;
    editUserFor: string;
    save: string;
    saving: string;
    updateSuccess: string;
    updateFail: string;
  };
  marketGroups: {
    createGroup: string;
    groupName: string;
    groupDesc: string;
    groupDescPlaceholder: string;
    createBtn: string;
    creating: string;
    noGroups: string;
    manage: string;
    collapse: string;
    edit: string;
    delete: string;
    save: string;
    cancel: string;
    deleteConfirm: string;
    marketsSection: string;
    saveMarkets: string;
    addUser: string;
    selectUser: string;
    addBtn: string;
    usersInGroup: string;
    removeUser: string;
    noUsers: string;
    loading: string;
    groupCount: string;
    optional: string;
  };
  commands: {
    pageTitle: string;
    createCommand: string;
    editCommand: string;
    commandName: string;
    description: string;
    descriptionPlaceholder: string;
    optional: string;
    handler: string;
    selectHandler: string;
    argsSection: string;
    noArgs: string;
    selectRequired: string;
    selectOptional: string;
    enabled: string;
    disabled: string;
    save: string;
    saving: string;
    cancel: string;
    edit: string;
    delete: string;
    deleteConfirm: string;
    deleteFail: string;
    saveFail: string;
    loading: string;
    cannotLoad: string;
    noCommands: string;
    commandCount: string;
    colName: string;
    colHandler: string;
    colArgs: string;
    colStatus: string;
    nameRequired: string;
    handlerRequired: string;
    argRequired: string;
  };
  commandGroups: {
    createGroup: string;
    groupName: string;
    groupDesc: string;
    groupDescPlaceholder: string;
    createBtn: string;
    createFail: string;
    noGroups: string;
    manage: string;
    collapse: string;
    edit: string;
    delete: string;
    save: string;
    cancel: string;
    saveFail: string;
    deleteFail: string;
    deleteConfirm: string;
    commandsSection: string;
    commandsLabel: string;
    saveCommands: string;
    saveCommandsFail: string;
    noCommandsAvailable: string;
    disabledLabel: string;
    addUser: string;
    selectUser: string;
    addBtn: string;
    addUserFail: string;
    usersInGroup: string;
    removeUser: string;
    noUsers: string;
    loading: string;
    cannotLoad: string;
    groupCount: string;
    optional: string;
  };
  monitoring: {
    pageTitle: string;
    generatedAt: string;
    refresh: string;
    loading: string;
    error: string;
    loginRequired: string;
    crawlSection: string;
    lastCrawl: string;
    staleness: string;
    never: string;
    dailyToday: string;
    intradayToday: string;
    staleWarning: string;
    marketClosed: string;
    predSection: string;
    lastPredict: string;
    todayTotal: string;
    expectedAlgos: string;
    missingAlgos: string;
    noMissing: string;
    algoTable: string;
    colAlgo: string;
    colTodayCount: string;
    colDirAccuracy: string;
    colReconciled: string;
    colCorrect: string;
    noAlgoData: string;
    botsSummary: string;
    totalBots: string;
    activeBots: string;
    byMarket: string;
    colMarket: string;
    colTrades: string;
    colWins: string;
    colLosses: string;
    colWinRate: string;
    colPnl: string;
    noBotsData: string;
    botsTable: string;
    colBotId: string;
    colAlgoBot: string;
    colWLBE: string;
    colReturnPct: string;
    colProfitFactor: string;
    colOpenPos: string;
    colUnrealized: string;
    returnPctNote: string;
    noBotsTable: string;
    sortAsc: string;
    sortDesc: string;
    allMarkets: string;
    filterAlgo: string;
    filterBotId: string;
    clearFilters: string;
    rowsPerPage: string;
    prevPage: string;
    nextPage: string;
    page: string;
  };
}
