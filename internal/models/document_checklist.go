package models

// DocumentChecklist - типы документов из чеклиста
type DocumentChecklist struct {
	ID          uint   `gorm:"primaryKey"`
	Code        string `gorm:"uniqueIndex;size:20"` // tz, rpz, source_code, etc.
	Name        string `gorm:"size:200"`            // Название документа
	Category    string `gorm:"size:100"`            // Категория (tz, eskd, espd, etc.)
	IsRequired  bool   `gorm:"default:true"`
	Description string `gorm:"type:text"`
}

// Список всех требуемых документов
var RequiredDocuments = []DocumentChecklist{
	// Задания и визируемые документы
	{Code: "tz", Name: "Техническое задание (ТЗ)", Category: "tasks", IsRequired: true, Description: "ГОСТ 34.602-89"},
	{Code: "title_pages", Name: "Титульные листы", Category: "tasks", IsRequired: true, Description: "ГОСТ Р 6.30-97"},
	{Code: "task", Name: "Учебное задание", Category: "tasks", IsRequired: true, Description: ""},

	// Конструкторская документация (ЕСКД)
	{Code: "structural_scheme", Name: "Схема структурная/функциональная", Category: "eskd", IsRequired: true, Description: "ГОСТ 2.701-84"},
	{Code: "rpz", Name: "РПЗ", Category: "eskd", IsRequired: true, Description: "Расчетно-пояснительная записка"},
	{Code: "electrical_scheme", Name: "Схема электрическая принципиальная", Category: "eskd", IsRequired: true, Description: "ГОСТ 2.701-84"},
	{Code: "pcb_drawing", Name: "Чертеж ПП", Category: "eskd", IsRequired: false, Description: ""},
	{Code: "assembly_drawing", Name: "Сборочный чертеж ПП", Category: "eskd", IsRequired: false, Description: ""},
	{Code: "specification", Name: "Спецификация", Category: "eskd", IsRequired: false, Description: "ГОСТ 2.101-68"},

	// Программная документация (ЕСПД)
	{Code: "algorithm_scheme", Name: "Схемы алгоритмов", Category: "espd", IsRequired: true, Description: "ГОСТ 19.701-90"},
	{Code: "software_structure", Name: "Схема структурная ПО", Category: "espd", IsRequired: true, Description: ""},
	{Code: "source_code", Name: "Исходные тексты программы", Category: "espd", IsRequired: true, Description: "ГОСТ 19.401-78"},
	{Code: "program_description", Name: "Описание программы", Category: "espd", IsRequired: true, Description: "ГОСТ 19.402-78"},

	// Эксплуатационная документация
	{Code: "user_manual", Name: "Руководство оператора", Category: "manuals", IsRequired: true, Description: ""},
	{Code: "admin_manual", Name: "Руководство сисадмина", Category: "manuals", IsRequired: true, Description: ""},
	{Code: "programmer_manual", Name: "Руководство программиста", Category: "manuals", IsRequired: false, Description: ""},

	// Моделирование и CAD
	{Code: "kompas_model", Name: "Модель Компас-3D (.a3d, .cdw)", Category: "cad", IsRequired: false, Description: "Компас-3D"},
	{Code: "proteus_project", Name: "Проект Proteus (.pdsprj, .dsn)", Category: "cad", IsRequired: false, Description: "Proteus"},
	{Code: "pcb_files", Name: "Файлы ПП (.sch, .pcb, .gbr)", Category: "cad", IsRequired: false, Description: "Altium, Eagle, KiCad"},
	{Code: "cad_3d", Name: "3D модели (.step, .stl)", Category: "cad", IsRequired: false, Description: "STEP, STL"},
	{Code: "makefile", Name: "MakeFiles и сборка", Category: "modeling", IsRequired: false, Description: ""},

	// Дополнительно
	{Code: "presentation", Name: "Презентация", Category: "other", IsRequired: true, Description: ""},
	{Code: "posters", Name: "Плакаты/Графические материалы", Category: "other", IsRequired: false, Description: ""},
	{Code: "video", Name: "Видео-руководства", Category: "other", IsRequired: false, Description: ""},
	{Code: "other", Name: "Другое", Category: "other", IsRequired: false, Description: ""},
}
