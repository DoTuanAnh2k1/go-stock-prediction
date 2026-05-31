package modelsdb

// AllModels - danh sách tất cả models để migrate
var AllModels = []interface{}{
	&Exchange{},
	&Stock{},
	&StockPrice{},
	&Prediction{},
	&SyncLog{},
	&GoldPrice{},
	&GoldPrediction{},
	&MacroIndicator{},
	&TrainingLog{},
	&User{},
	&CronSchedule{},
}
