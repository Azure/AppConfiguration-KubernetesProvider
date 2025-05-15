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
	fileStyleSettingString1, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "key1=value1", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	emptyString := ""
	fileStyleSettingString2, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"empty": &emptyString,
		},
		IsJsonContentTypeMap: map[string]bool{
			"empty": false,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "empty=", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	jsonKeyValue := "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString3, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"jsonKey": &jsonKeyValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"jsonKey": true,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "jsonKey={\"vKey1\":{\"vKey2\":\"value3\"}}", fileStyleSettingString3["test"])
	assert.Nil(t, err)

	fileStyleSettingString4, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"jsonKey": &jsonKeyValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"jsonKey": false,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "jsonKey={\"vKey1\":{\"vKey2\":\"value3\"}}", fileStyleSettingString4["test"])
	assert.Nil(t, err)

	doublequotationValue := "\"\""
	fileStyleSettingString5, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"doublequotation": &doublequotationValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"doublequotation": false,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "doublequotation=\"\"", fileStyleSettingString5["test"])
	assert.Nil(t, err)

	multipleLineJsonValue := "{\n\t\"a\":\"json\",\n\t\"b\":{\n\t\t\"c\":{\n\t\t\t\"d\": \"test\"\n\t\t},\n\t\t\"f\":[1,2,3]\n\t}\n}"
	fileStyleSettingString6, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"multipleLineJson": &multipleLineJsonValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"multipleLineJson": true,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Equal(t, "multipleLineJson={\n\t\"a\":\"json\",\n\t\"b\":{\n\t\t\"c\":{\n\t\t\t\"d\": \"test\"\n\t\t},\n\t\t\"f\":[1,2,3]\n\t}\n}", fileStyleSettingString6["test"])
	assert.Nil(t, err)

	fileStyleSettingString7, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"key1": nil,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "test",
	})
	assert.Empty(t, fileStyleSettingString7["test"])
	assert.Nil(t, err)
}

func TestCreateFileStyleSettings(t *testing.T) {
	fileStyleSettingString1, err := createTypedSettings(&RawSettings{KeyValueSettings: make(map[string]*string), IsJsonContentTypeMap: make(map[string]bool)}, &acpv1.DataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.Empty(t, fileStyleSettingString1["test"])
	assert.Nil(t, err)
}

func TestConfigMapDataOptionsIsNull(t *testing.T) {
	key1Value := "value1"
	key2_subkey1Value := "1"
	emptyValue := ""
	key3Value := "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	settings, err := createTypedSettings(&RawSettings{
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
	fileStyleSettingString1, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.Equal(t, "empty: \"\"\nkey1: value1\nkey2_subkey1: \"1\"\nkey3:\n    vKey1:\n        vKey2: value3\n", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	key1Value = "[\"value1\",\"value2\"]"
	key2Value := "\"value2\""
	key3Value = "\"value3\""
	fileStyleSettingString2, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type: acpv1.Yaml,
		Key:  "test",
	})
	assert.Equal(t, "key1:\n    - value1\n    - value2\nkey2: '\"value2\"'\nkey3: value3\n", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	key1Value = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString3, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
			"key2": nil,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
			"key2": false,
		},
	}, &acpv1.DataOptions{
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
	fileStyleSettingString1, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type: acpv1.Json,
		Key:  "test",
	})
	assert.Equal(t, "{\"empty\":\"\",\"key1\":\"value1\",\"key2_subkey1\":\"1\",\"key3\":{\"vKey1\":{\"vKey2\":\"value3\"}},\"key4\":null}", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	key1Value = "[\"value1\",\"value2\"]"
	key2Value := "\"value2\""
	key3Value = "\"value3\""
	fileStyleSettingString2, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type: acpv1.Json,
		Key:  "test",
	})
	assert.Equal(t, "{\"key1\":[\"value1\",\"value2\"],\"key2\":\"\\\"value2\\\"\",\"key3\":\"value3\"}", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	key1Value = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString3, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"key1": &key1Value,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1": false,
		},
	}, &acpv1.DataOptions{
		Type: acpv1.Json,
		Key:  "test",
	})
	assert.Equal(t, "{\"key1\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}", fileStyleSettingString3["test"])
	assert.Nil(t, err)

	jsonStringValue := "\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\""
	fileStyleSettingString4, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"jsonString": &jsonStringValue,
		},
		IsJsonContentTypeMap: map[string]bool{
			"jsonString": true,
		},
	}, &acpv1.DataOptions{
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
	fileStyleSettingString1, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"empty\":\"\",\"key1\":{\"subKey1\":\"value1\",\"subKey2\":\"value2\",\"subKey3\":[{\"test1\":\"value4\"},{\"test2\":\"value5\"}]},\"key2\":{\"subkey1\":\"1\"},\"key3\":{\"vKey1\":{\"vKey2\":\"value3\"}},\"key4\":null}", fileStyleSettingString1["test"])
	assert.Nil(t, err)

	fileStyleSettingString2, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"empty\":\"\",\"key1\":{\"subKey1\":\"value1\",\"subKey2\":\"value2\",\"subKey3\":{\"0\":{\"test1\":\"value4\",\"test2\":null},\"test2\":{\"test3\":\"value5\"}}},\"key2\":{\"subkey1\":\"1\"},\"key3\":{\"vKey1\":{\"vKey2\":\"value3\"}}}", fileStyleSettingString2["test"])
	assert.Nil(t, err)

	fileStyleSettingString3, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Yaml,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "empty: \"\"\nkey1:\n    subKey1: value1\n    subKey2: value2\n    subKey3:\n        \"0\":\n            test1: value4\n        test2:\n            test3: value5\nkey2:\n    subkey1: \"1\"\nkey3:\n    vKey1:\n        vKey2: value3\n", fileStyleSettingString3["test"])
	assert.Nil(t, err)

	value1 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	value2 = "value5"
	value3 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString4, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey3\":[{\"test1\":{\"vKey1\":{\"vKey2\":\"value3\"}}},{\"test3\":\"value5\"}]},\"key3\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}", fileStyleSettingString4["test"])
	assert.Nil(t, err)

	anotherDelimiter := "_"
	fileStyleSettingString5, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &anotherDelimiter,
	})

	assert.Equal(t, "{\"key1.subKey3.0.test1\":{\"vKey1\":{\"vKey2\":\"value3\"}},\"key1.subKey3.1.test3\":\"value5\",\"key3\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}", fileStyleSettingString5["test"])
	assert.Nil(t, err)

	value1 = "value1"
	value2 = "value2"
	value3 = "value6"
	fileStyleSettingString6, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &anotherDelimiter,
	})

	assert.Equal(t, "{\"\":{\"key1\":{\"subKey1\":\"value1\"}},\".key3\":{\"key4.key5\":{\"key6\":\"value6\"}},\"key2\":{\"\":{\"\":{\"test\":{\"\":\"value2\"}}}}}", fileStyleSettingString6["test"])
	assert.Nil(t, err)

	fileStyleSettingString7, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"key1_2": &value1,
			"key1_1": &value2,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1_2": false,
			"key1_1": false,
		},
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &anotherDelimiter,
	})

	assert.Equal(t, "{\"key1\":{\"1\":\"value2\",\"2\":\"value1\"}}", fileStyleSettingString7["test"])
	assert.Nil(t, err)

	value1 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	value2 = "value5"
	value3 = "{\"vKey1\":{\"vKey2\":\"value3\"}}"
	fileStyleSettingString8, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey3\":[{\"test1\":{\"vKey1\":{\"vKey2\":\"value3\"}}},{\"test3\":\"value5\"}]},\"key3\":{\"subKey4\":[{\"test1\":\"{\\\"vKey1\\\":{\\\"vKey2\\\":\\\"value3\\\"}}\"}]}}", fileStyleSettingString8["test"])
	assert.Nil(t, err)

	value1 = "{\"vKey1\":\"value1\", \"vKey3\":\"value5\"}"
	value2 = "value4"
	value3 = "value3"
	fileStyleSettingString9, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
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
	fileStyleSettingString10, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey1\":{\"vKey1\":\"value3\",\"vKey2\":\"value4\"}}}", fileStyleSettingString10["test"])
	assert.Nil(t, err)

	value1 = "[\"test1\", \"test2\"]"
	value2 = "value3"
	value3 = "value4"
	fileStyleSettingString11, err := createTypedSettings(&RawSettings{
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
	}, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "test",
		Separator: &delimiter,
	})

	assert.Equal(t, "{\"key1\":{\"subKey1\":{\"0\":\"test1\",\"1\":\"test2\",\"vKey1\":\"value3\",\"vKey2\":\"value4\"}}}", fileStyleSettingString11["test"])
	assert.Nil(t, err)

	value1 = "[{\"vKey1\":\"test1\"},{\"vKey2\":\"test2\"}]"
	value2 = "value3"
	fileStyleSettingString12, err := createTypedSettings(&RawSettings{
		KeyValueSettings: map[string]*string{
			"key1.subKey1":         &value1,
			"key1.subKey1.0.vKey1": &value2,
		},
		IsJsonContentTypeMap: map[string]bool{
			"key1.subKey1":         true,
			"key1.subKey1.0.vKey1": false,
		},
	}, &acpv1.DataOptions{
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

func TestCreateTypedSecrets(t *testing.T) {
	// Test with empty secrets map
	emptySecrets := make(map[string]string)
	result, err := createTypedSecrets(emptySecrets, nil)
	assert.Empty(t, result)
	assert.Nil(t, err)

	// Create test secrets map
	secretsMap := map[string]string{
		"secretKey1":  "secretValue1",
		"secretKey2":  "secretValue2",
		"jsonSecret":  "{\"key\":\"value\"}",
		"emptySecret": "",
	}

	// Test with nil dataOptions (default behavior)
	result, err = createTypedSecrets(secretsMap, nil)
	assert.Nil(t, err)
	assert.Equal(t, len(secretsMap), len(result))
	assert.Equal(t, "secretValue1", result["secretKey1"])
	assert.Equal(t, "secretValue2", result["secretKey2"])
	assert.Equal(t, "{\"key\":\"value\"}", result["jsonSecret"])
	assert.Equal(t, "", result["emptySecret"])

	// Test with explicit Default type
	result, err = createTypedSecrets(secretsMap, &acpv1.DataOptions{
		Type: acpv1.Default,
	})
	assert.Nil(t, err)
	assert.Equal(t, len(secretsMap), len(result))
	assert.Equal(t, "secretValue1", result["secretKey1"])
	assert.Equal(t, "secretValue2", result["secretKey2"])
	assert.Equal(t, "{\"key\":\"value\"}", result["jsonSecret"])
	assert.Equal(t, "", result["emptySecret"])

	// Test with Properties type
	result, err = createTypedSecrets(secretsMap, &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "secrets.properties",
	})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
	// Properties order might vary, so check for each key-value pair
	assert.Contains(t, result["secrets.properties"], "secretKey1=secretValue1")
	assert.Contains(t, result["secrets.properties"], "secretKey2=secretValue2")

	// Test with JSON type
	result, err = createTypedSecrets(secretsMap, &acpv1.DataOptions{
		Type: acpv1.Json,
		Key:  "secrets.json",
	})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
	// The order of JSON keys is unpredictable, so use JSONEq for comparison
	assert.JSONEq(t, `{
		"secretKey1":"secretValue1", 
		"secretKey2":"secretValue2", 
		"jsonSecret":"{\"key\":\"value\"}",
		"emptySecret":""
	}`, result["secrets.json"])

	// Test with YAML type
	result, err = createTypedSecrets(secretsMap, &acpv1.DataOptions{
		Type: acpv1.Yaml,
		Key:  "secrets.yaml",
	})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
	// YAML format is harder to compare exactly, so check for key portions
	assert.Equal(t, result["secrets.yaml"], "emptySecret: \"\"\njsonSecret: '{\"key\":\"value\"}'\nsecretKey1: secretValue1\nsecretKey2: secretValue2\n")

	// Test with hierarchical structure using separator
	hierarchicalSecrets := map[string]string{
		"database.credentials.username": "admin",
		"database.credentials.password": "secret123",
		"api.key":                       "api-key-value",
		"standalone":                    "standalone-value",
	}

	separator := "."
	result, err = createTypedSecrets(hierarchicalSecrets, &acpv1.DataOptions{
		Type:      acpv1.Json,
		Key:       "config.json",
		Separator: &separator,
	})

	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))

	// Expected hierarchical structure in JSON format
	expectedJson := `{
		"database": {
			"credentials": {
				"username": "admin",
				"password": "secret123"
			}
		},
		"api": {
			"key": "api-key-value"
		},
		"standalone": "standalone-value"
	}`

	assert.JSONEq(t, expectedJson, result["config.json"])
}

func TestIsAIConfigurationContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType *string
		expected    bool
	}{
		{
			name:        "valid AI configuration content type",
			contentType: strPtr("application/json; profile=\"https://azconfig.io/mime-profiles/ai\""),
			expected:    true,
		},
		{
			name:        "valid AI configuration content type with extra parameters",
			contentType: strPtr("application/json; charset=utf-8; profile=\"https://azconfig.io/mime-profiles/ai\"; param=value"),
			expected:    true,
		},
		{
			name:        "invalid AI configuration content type - missing profile keyword",
			contentType: strPtr("application/json; \"https://azconfig.io/mime-profiles/ai\""),
			expected:    false,
		},
		{
			name:        "invalid content type - wrong profile",
			contentType: strPtr("application/json; profile=\"https://azconfig.io/mime-profiles/other\""),
			expected:    false,
		},
		{
			name:        "invalid content type - partial match",
			contentType: strPtr("application/json; profile=\"prefix-https://azconfig.io/mime-profiles/ai\""),
			expected:    false,
		},
		{
			name:        "invalid content type - not JSON",
			contentType: strPtr("text/plain; profile=\"https://azconfig.io/mime-profiles/ai\""),
			expected:    false,
		},
		{
			name:        "empty content type",
			contentType: strPtr(""),
			expected:    false,
		},
		{
			name:        "nil content type",
			contentType: nil,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isJsonContentType(tt.contentType) && isAIConfigurationContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("isAIConfigurationContentType(%v) = %v, want %v",
					tt.contentType, result, tt.expected)
			}
		})
	}
}

func TestIsAIChatCompletionContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType *string
		expected    bool
	}{
		{
			name:        "valid AI chat completion content type",
			contentType: strPtr("application/json; profile=\"https://azconfig.io/mime-profiles/ai/chat-completion\""),
			expected:    true,
		},
		{
			name:        "valid AI chat completion with multiple parameters",
			contentType: strPtr("application/json; charset=utf-8; profile=\"https://azconfig.io/mime-profiles/ai/chat-completion\"; param=value"),
			expected:    true,
		},
		{
			name:        "invalid content type - missing profile keyword",
			contentType: strPtr("application/json; \"https://azconfig.io/mime-profiles/ai/chat-completion\""),
			expected:    false,
		},
		{
			name:        "invalid content type - wrong profile",
			contentType: strPtr("application/json; profile=\"https://azconfig.io/mime-profiles/other\""),
			expected:    false,
		},
		{
			name:        "invalid content type - partial match",
			contentType: strPtr("application/json; profile=\"prefix-https://azconfig.io/mime-profiles/ai/chat-completion\""),
			expected:    false,
		},
		{
			name:        "invalid content type - not JSON",
			contentType: strPtr("text/plain; profile=\"https://azconfig.io/mime-profiles/ai/chat-completion\""),
			expected:    false,
		},
		{
			name:        "JSON content type without AI chat completion profile",
			contentType: strPtr("application/json"),
			expected:    false,
		},
		{
			name:        "empty content type",
			contentType: strPtr(""),
			expected:    false,
		},
		{
			name:        "nil content type",
			contentType: nil,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isJsonContentType(tt.contentType) && isAIChatCompletionContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("isAIChatCompletionContentType(%v) = %v, want %v",
					tt.contentType, result, tt.expected)
			}
		})
	}
}

// Helper function to create string pointers for tests
func strPtr(s string) *string {
	return &s
}
