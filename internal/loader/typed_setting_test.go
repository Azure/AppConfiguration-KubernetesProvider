// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshalProperties(t *testing.T) {
	properties1 := marshalProperties(nil)
	assert.Empty(t, properties1)

	properties2 := marshalProperties(make(map[string]string))
	assert.Empty(t, properties2)

	properties3 := marshalProperties(map[string]string{
		"key1": "value1",
	})
	assert.Equal(t, "key1=value1", properties3)

	properties4 := marshalProperties(map[string]string{
		"key3": "{\"vKey1\":{\"vKey2\":\"value3\"}}",
	})
	assert.Equal(t, "key3={\"vKey1\":{\"vKey2\":\"value3\"}}", properties4)
}

func TestIsJsonContentType(t *testing.T) {
	string1 := "application/json"
	assert.True(t, isJsonContentType(&string1))
	string2 := "application/activity+json"
	assert.True(t, isJsonContentType(&string2))
	string3 := "application/vnd.foobar+json;charset=utf-8"
	assert.True(t, isJsonContentType(&string3))
	string4 := ""
	assert.False(t, isJsonContentType(&string4))
	string5 := " "
	assert.False(t, isJsonContentType(&string5))
	string6 := "json"
	assert.False(t, isJsonContentType(&string6))
	string7 := "application"
	assert.False(t, isJsonContentType(&string7))
	string8 := "application/test"
	assert.False(t, isJsonContentType(&string8))
	string9 := "+"
	assert.False(t, isJsonContentType(&string9))
	string10 := "vnd.foobar+json;charset=utf-8"
	assert.False(t, isJsonContentType(&string10))
	string11 := "someteststring"
	assert.False(t, isJsonContentType(&string11))
	string12 := "application/json+"
	assert.False(t, isJsonContentType(&string12))
	string13 := "application/json+test"
	assert.False(t, isJsonContentType(&string13))
}

func TestCreateFileStyleSettingsWithPropertiesFormat(t *testing.T) {
	key1Value := "value1"
	fileStyleSettingString1, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "key1=value1", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	emptyString := ""
	fileStyleSettingString2, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"empty": &emptyString,
		},
		IsJsonContentTypeMap: map[string]bool{
			"empty": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "empty=", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	jsonKeyValue := "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString3, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"jsonKey": &jsonKeyValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"jsonKey": true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "jsonKey={\"vKey1\":{\"vKey2\":\"value3\"}}", fileStyleSettingString3["test"])
	assert.Nil(t, err)

	fileStyleSettingString4, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"jsonKey": &jsonKeyValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"jsonKey": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "jsonKey={\"vKey1\":{\"vKey2\":\"value3\"}}", fileStyleSettingString4["test"])
	assert.Nil(t, err)

	doublequotationValue := "\"\""
	fileStyleSettingString5, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"doublequotation": &doublequotationValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"doublequotation": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "doublequotation=\"\"", fileStyleSettingString5["test"])
	assert.Nil(t, err)

	multipleLineJsonValue := "{\n\t\"a\":\"json\",\n\t\"b\":{\n\t\t\"c\":{\n\t\t\t\"d\": \"test\"\n\t\t},\n\t\t\"f\":[1,2,3]\n\t}\n}"
	fileStyleSettingString6, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"multipleLineJson": &multipleLineJsonValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"multipleLineJson": true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "multipleLineJson={\n\t\"a\":\"json\",\n\t\"b\":{\n\t\t\"c\":{\n\t\t\t\"d\": \"test\"\n\t\t},\n\t\t\"f\":[1,2,3]\n\t}\n}", fileStyleSettingString6["test"])
	assert.Nil(t, err)

	fileStyleSettingString7, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1": nil,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Empty(t, fileStyleSettingString7["test"])
	assert.Nil(t, err)
}

func TestCreateFileStyleSettings(t *testing.T) {
	fileStyleSettingString1, err := createTypedSettings(&UnformattedSettings{KeyValueSettings: make(map[string]*string), IsJsonContentTypeMap: make(map[string]bool)}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.Empty(t, fileStyleSettingString1["test"])
	assert.Nil(t, err)
}

func TestPassedSettingIsNull(t *testing.T) {
	fileStyleSettingString1, err := createTypedSettings(&UnformattedSettings{}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.NotNil(t, err)
	assert.Nil(t, fileStyleSettingString1)
}

func TestConfigMapDataOptionsIsNull(t *testing.T) {
	key1Value := "value1"
	key2_subkey1Value := "1"
	emptyValue := ""
	key3Value := "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	settings, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1":         &key1Value,
			"key2_subkey1": &key2_subkey1Value,
			"empty":        &emptyValue,
			"key3":         &key3Value,
			"key4":         nil,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1":         false,
			"key2_subkey1": false,
			"empty":        false,
			"key3":         true,
			"key4":         false,
		},
	}, nil)
	assert.Nil(t, err)
	assert.Equal(t, "value1", settings["key1"])
	assert.Equal(t, "1", settings["key2_subkey1"])
	assert.Equal(t, "", settings["empty"])
	assert.Equal(t, "{\"vKey1\":{\"vKey2\":\"value3\"}}", settings["key3"])
	assert.Equal(t, "", settings["key4"])
}

func TestCreateFileStyleSettingsWithYamlFormat(t *testing.T) {
	key1Value := "value1"
	key2_subkey1Value := "1"
	emptyValue := ""
	key3Value := "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString1, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1":         &key1Value,
			"key2_subkey1": &key2_subkey1Value,
			"empty":        &emptyValue,
			"key3":         &key3Value,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1":         false,
			"key2_subkey1": false,
			"empty":        false,
			"key3":         true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.Equal(t, "empty: \"\"\nkey1: value1\nkey2_subkey1: \"1\"\nkey3:\n    vKey1:\n        vKey2: value3\n", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	key1Value = "[\"value1\",\"value2\"]"
	key2Value := "\"value2\""
	key3Value = "\"value3\""
	fileStyleSettingString2, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
			"key2": &key2Value,
			"key3": &key3Value,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": true,
			"key2": false,
			"key3": true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.Equal(t, "key1:\n    - value1\n    - value2\nkey2: '\"value2\"'\nkey3: value3\n", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	key1Value = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString3, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
			"key2": nil,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
			"key2": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.Equal(t, "key1: '{\"vKey1\":{\"vKey2\":\"value3\"}}'\nkey2: null\n", fileStyleSettingString3["test"])
	assert.Nil(t, err)
}

func TestCreateFileStyleSettingsWithJsonFormat(t *testing.T) {
	key1Value := "value1"
	key2_subkey1Value := "1"
	emptyValue := ""
	key3Value := "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString1, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1":         &key1Value,
			"key2_subkey1": &key2_subkey1Value,
			"empty":        &emptyValue,
			"key3":         &key3Value,
			"key4":         nil,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1":         false,
			"key2_subkey1": false,
			"empty":        false,
			"key3":         true,
			"key4":         false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Json,
		Key:  "test",
	})
	assert.Equal(t, "{\"empty\":\"\",\"key1\":\"value1\",\"key2_subkey1\":\"1\",\"key3\":{\"vKey1\":{\"vKey2\":\"value3\"}},\"key4\":null}", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	key1Value = "[\"value1\",\"value2\"]"
	key2Value := "\"value2\""
	key3Value = "\"value3\""
	fileStyleSettingString2, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
			"key2": &key2Value,
			"key3": &key3Value,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": true,
			"key2": false,
			"key3": true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Json,
		Key:  "test",
	})
	assert.Equal(t, "{\"key1\":[\"value1\",\"value2\"],\"key2\":\"\\\"value2\\\"\",\"key3\":\"value3\"}", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	key1Value = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString3, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Json,
		Key:  "test",
	})
	assert.Equal(t, "{\"key1\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}", fileStyleSettingString3["test"])
	assert.Nil(t, err)

	jsonStringValue := "\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\""
	fileStyleSettingString4, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"jsonString": &jsonStringValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"jsonString": true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type: acpv1.Json,
		Key:  "test",
	})
	assert.Equal(t, "{\"jsonString\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}", fileStyleSettingString4["test"])
	assert.Nil(t, err)
}

func TestBuildHierarchicalSetting(t *testing.T) {
	delimiter := "."
	value1 := "value1"
	value2 := "value2"
	value3 := "value4"
	value4 := "value5"
	value5 := "1"
	value6 := ""
	value7 := "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString1, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":         &value1,
			"key1.subKey2":         &value2,
			"key1.subKey3.0.test1": &value3,
			"key1.subKey3.1.test2": &value4,
			"key2.subkey1":         &value5,
			"empty":                &value6,
			"key3":                 &value7,
			"key4":                 nil,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1":         false,
			"key2_subkey1": false,
			"empty":        false,
			"key3":         true,
			"key4":         false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"empty\":\"\",\"key1\":{\"subKey1\":\"value1\",\"subKey2\":\"value2\",\"subKey3\":[{\"test1\":\"value4\"},{\"test2\":\"value5\"}]},\"key2\":{\"subkey1\":\"1\"},\"key3\":{\"vKey1\":{\"vKey2\":\"value3\"}},\"key4\":null}", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	fileStyleSettingString2, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":             &value1,
			"key1.subKey2":             &value2,
			"key1.subKey3.0.test1":     &value3,
			"key1.subKey3.test2.test3": &value4,
			"key1.subKey3.0.test2":     nil,
			"key2.subkey1":             &value5,
			"empty":                    &value6,
			"key3":                     &value7,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1":         false,
			"key2_subkey1": false,
			"empty":        false,
			"key3":         true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"empty\":\"\",\"key1\":{\"subKey1\":\"value1\",\"subKey2\":\"value2\",\"subKey3\":{\"0\":{\"test1\":\"value4\",\"test2\":null},\"test2\":{\"test3\":\"value5\"}}},\"key2\":{\"subkey1\":\"1\"},\"key3\":{\"vKey1\":{\"vKey2\":\"value3\"}}}", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	fileStyleSettingString3, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":             &value1,
			"key1.subKey2":             &value2,
			"key1.subKey3.0.test1":     &value3,
			"key1.subKey3.test2.test3": &value4,
			"key2.subkey1":             &value5,
			"empty":                    &value6,
			"key3":                     &value7,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1":         false,
			"key2_subkey1": false,
			"empty":        false,
			"key3":         true,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Yaml,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "empty: \"\"\nkey1:\n    subKey1: value1\n    subKey2: value2\n    subKey3:\n        \"0\":\n            test1: value4\n        test2:\n            test3: value5\nkey2:\n    subkey1: \"1\"\nkey3:\n    vKey1:\n        vKey2: value3\n", fileStyleSettingString3["test"])
	assert.Nil(t, err)

	value1 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	value2 = "value5"
	value3 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString4, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey3.0.test1": &value1,
			"key1.subKey3.1.test3": &value2,
			"key3":                 &value3,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key3":                 false,
			"key1.subKey3.0.test1": true,
			"key1.subKey3.1.test3": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey3\":[{\"test1\":{\"vKey1\":{\"vKey2\":\"value3\"}}},{\"test3\":\"value5\"}]},\"key3\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}", fileStyleSettingString4["test"])
	assert.Nil(t, err)

	anotherDelimiter := "_"
	fileStyleSettingString5, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey3.0.test1": &value1,
			"key1.subKey3.1.test3": &value2,
			"key3":                 &value3,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key3":                 false,
			"key1.subKey3.0.test1": true,
			"key1.subKey3.1.test3": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &anotherDelimiter,
	})

	assert.Equal(t, "{\"key1.subKey3.0.test1\":{\"vKey1\":{\"vKey2\":\"value3\"}},\"key1.subKey3.1.test3\":\"value5\",\"key3\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}", fileStyleSettingString5["test"])
	assert.Nil(t, err)

	value1 = "value1"
	value2 = "value2"
	value3 = "value6"
	fileStyleSettingString6, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"_key1_subKey1":        &value1,
			"key2___test_":         &value2,
			".key3_key4.key5_key6": &value3,
		},
		IsJsonContentTypeMap: map[string]bool{
			"_key1_subKey1":        false,
			"key2___test_":         false,
			".key3_key4.key5_key6": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &anotherDelimiter,
	})

	assert.Equal(t, "{\"\":{\"key1\":{\"subKey1\":\"value1\"}},\".key3\":{\"key4.key5\":{\"key6\":\"value6\"}},\"key2\":{\"\":{\"\":{\"test\":{\"\":\"value2\"}}}}}", fileStyleSettingString6["test"])
	assert.Nil(t, err)

	fileStyleSettingString7, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1_2": &value1,
			"key1_1": &value2,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1_2": false,
			"key1_1": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &anotherDelimiter,
	})

	assert.Equal(t, "{\"key1\":{\"1\":\"value2\",\"2\":\"value1\"}}", fileStyleSettingString7["test"])
	assert.Nil(t, err)

	value1 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	value2 = "value5"
	value3 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString8, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey3.0.test1": &value1,
			"key1.subKey3.1.test3": &value2,
			"key3.subKey4.0.test1": &value3,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key3":                 false,
			"key1.subKey3.0.test1": true,
			"key1.subKey3.1.test3": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey3\":[{\"test1\":{\"vKey1\":{\"vKey2\":\"value3\"}}},{\"test3\":\"value5\"}]},\"key3\":{\"subKey4\":[{\"test1\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}]}}", fileStyleSettingString8["test"])
	assert.Nil(t, err)

	value1 = "{\"vKey1\":\"value1\", \"vKey3\":\"value5\"}"
	value2 = "value4"
	value3 = "value3"
	fileStyleSettingString9, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":       &value1,
			"key1.subKey1.vKey2": &value2,
			"key1.subKey1.vKey1": &value3,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1.subKey1":       true,
			"key1.subKey1.vKey1": false,
			"key1.subKey1.vKey2": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	// AppConfig don't guarantee the result order and Golang can't guarantee the order of map, so the result may be different.
	if fileStyleSettingString9["test"] != "{\"key1\":{\"subKey1\":{\"vKey1\":\"value3\",\"vKey2\":\"value4\",\"vKey3\":\"value5\"}}}" {
		assert.Equal(t, "{\"key1\":{\"subKey1\":{\"vKey1\":\"value1\",\"vKey2\":\"value4\",\"vKey3\":\"value5\"}}}", fileStyleSettingString9["test"])
	}
	assert.Nil(t, err)

	value1 = "{}"
	value2 = "value3"
	value3 = "value4"
	fileStyleSettingString10, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":       &value1,
			"key1.subKey1.vKey1": &value2,
			"key1.subKey1.vKey2": &value3,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1.subKey1":       true,
			"key1.subKey1.vKey1": false,
			"key1.subKey1.vKey2": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey1\":{\"vKey1\":\"value3\",\"vKey2\":\"value4\"}}}", fileStyleSettingString10["test"])
	assert.Nil(t, err)

	value1 = "[\"test1\", \"test2\"]"
	value2 = "value3"
	value3 = "value4"
	fileStyleSettingString11, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":       &value1,
			"key1.subKey1.vKey1": &value2,
			"key1.subKey1.vKey2": &value3,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1.subKey1":       true,
			"key1.subKey1.vKey1": false,
			"key1.subKey1.vKey2": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey1\":{\"0\":\"test1\",\"1\":\"test2\",\"vKey1\":\"value3\",\"vKey2\":\"value4\"}}}", fileStyleSettingString11["test"])
	assert.Nil(t, err)

	value1 = "[{\"vKey1\":\"test1\"},{\"vKey2\":\"test2\"}]"
	value2 = "value3"
	fileStyleSettingString12, err := createTypedSettings(&UnformattedSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":         &value1,
			"key1.subKey1.0.vKey1": &value2,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1.subKey1":         true,
			"key1.subKey1.0.vKey1": false,
		},
	}, &acpv1.ConfigMapDataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	// AppConfig don't guarantee the result order and Golang can't guarantee the order of map, so the result may be different.
	if fileStyleSettingString12["test"] != "{\"key1\":{\"subKey1\":[{\"vKey1\":\"test1\"},{\"vKey2\":\"test2\"}]}}" {
		assert.Equal(t, "{\"key1\":{\"subKey1\":[{\"vKey1\":\"value3\"},{\"vKey2\":\"test2\"}]}}", fileStyleSettingString12["test"])
	}
	assert.Nil(t, err)
}
