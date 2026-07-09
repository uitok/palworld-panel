package gvas

import "fmt"

var typeHints = map[string]string{
	".worldSaveData.CharacterContainerSaveData.Key":                                                                  "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Key":                                                                   "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Value":                                                                 "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Key":                                                                      "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value":                                                     "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Key":                                 "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Value":                               "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value":                                                                    "StructProperty",
	".worldSaveData.ItemContainerSaveData.Key":                                                                       "StructProperty",
	".worldSaveData.MapObjectSaveData.MapObjectSaveData.ConcreteModel.ModuleMap.Value":                               "StructProperty",
	".worldSaveData.MapObjectSaveData.MapObjectSaveData.Model.EffectMap.Value":                                       "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Key":                                                             "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value":                                                           "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Key":                 "Guid",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value":               "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value.ItemMap.Value": "StructProperty",
	".worldSaveData.WorkSaveData.WorkSaveData.WorkAssignMap.Value":                                                   "StructProperty",
	".worldSaveData.BaseCampSaveData.Key":                                                                            "Guid",
	".worldSaveData.BaseCampSaveData.Value":                                                                          "StructProperty",
	".worldSaveData.BaseCampSaveData.Value.ModuleMap.Value":                                                          "StructProperty",
	".worldSaveData.ItemContainerSaveData.Value":                                                                     "StructProperty",
	".worldSaveData.CharacterContainerSaveData.Value":                                                                "StructProperty",
	".worldSaveData.GroupSaveDataMap.Key":                                                                            "Guid",
	".worldSaveData.GroupSaveDataMap.Value":                                                                          "StructProperty",
	".worldSaveData.EnemyCampSaveData.EnemyCampStatusMap.Value":                                                      "StructProperty",
	".worldSaveData.InvaderSaveData.Key":                                                                             "Guid",
	".worldSaveData.InvaderSaveData.Value":                                                                           "StructProperty",
	".worldSaveData.OilrigSaveData.OilrigMap.Value":                                                                  "StructProperty",
	".worldSaveData.SupplySaveData.SupplyInfos.Key":                                                                  "Guid",
	".worldSaveData.SupplySaveData.SupplyInfos.Value":                                                                "StructProperty",
}

func (r *Reader) PropertiesUntilEnd(path string) (map[string]any, error) {
	props := map[string]any{}
	for {
		if r.EOF() {
			return nil, fmt.Errorf("unexpected EOF while reading properties at %s", path)
		}
		name, err := r.FString()
		if err != nil {
			return nil, err
		}
		if name == "None" {
			break
		}
		typeName, err := r.FString()
		if err != nil {
			return nil, err
		}
		size, err := r.U64()
		if err != nil {
			return nil, err
		}
		prop, err := r.Property(typeName, int64(size), path+"."+name, false)
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", path+"."+name, typeName, err)
		}
		props[name] = prop
	}
	return props, nil
}

func (r *Reader) Property(typeName string, size int64, path string, nestedCustom bool) (map[string]any, error) {
	if !nestedCustom {
		if path == ".worldSaveData.GroupSaveDataMap" {
			return r.groupSaveDataMap(typeName, size, path)
		}
		if path == ".worldSaveData.CharacterSaveParameterMap.Value.RawData" {
			return r.byteArrayRawData(typeName, size, path, decodeCharacterRaw)
		}
		if path == ".worldSaveData.BaseCampSaveData.Value.RawData" {
			return r.byteArrayRawData(typeName, size, path, decodeBaseCampRaw)
		}
	}

	var value map[string]any
	switch typeName {
	case "StructProperty":
		v, err := r.Struct(path)
		if err != nil {
			return nil, err
		}
		value = v
	case "IntProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.I32()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": int(v)}
	case "Int8Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.Byte()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": int(int8(v))}
	case "UInt16Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.U16()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": int(v)}
	case "UInt32Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.U32()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": uint(v)}
	case "UInt64Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.U64()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": v}
	case "Int64Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.I64()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": v}
	case "FixedPoint64Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.I32()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": int(v)}
	case "FloatProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.Float()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": v}
	case "DoubleProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.Double()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": v}
	case "StrProperty", "NameProperty", "ObjectProperty", "SoftObjectProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.FString()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": v}
	case "EnumProperty":
		enumType, err := r.FString()
		if err != nil {
			return nil, err
		}
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		enumValue, err := r.FString()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": map[string]any{"type": enumType, "value": enumValue}}
	case "BoolProperty":
		v, err := r.Bool()
		if err != nil {
			return nil, err
		}
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": v}
	case "ByteProperty":
		enumType, err := r.FString()
		if err != nil {
			return nil, err
		}
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		var enumValue any
		if enumType == "None" {
			enumValue, err = r.Byte()
		} else {
			enumValue, err = r.FString()
		}
		if err != nil {
			return nil, err
		}
		value = map[string]any{"id": id, "value": map[string]any{"type": enumType, "value": enumValue}}
	case "ArrayProperty":
		arrayType, err := r.FString()
		if err != nil {
			return nil, err
		}
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		v, err := r.ArrayProperty(arrayType, size-4, path)
		if err != nil {
			return nil, err
		}
		value = map[string]any{"array_type": arrayType, "id": id, "value": v}
	case "MapProperty":
		v, err := r.MapProperty(path)
		if err != nil {
			return nil, err
		}
		value = v
	case "SetProperty":
		elemType, err := r.FString()
		if err != nil {
			return nil, err
		}
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, err
		}
		if _, err := r.U32(); err != nil {
			return nil, err
		}
		count, err := r.U32()
		if err != nil {
			return nil, err
		}
		values := make([]any, 0, count)
		for range count {
			v, err := r.PropValue(elemType, "", path+".Value")
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		value = map[string]any{"element_type": elemType, "id": id, "value": values}
	default:
		return nil, fmt.Errorf("unknown property type %q", typeName)
	}
	value["type"] = typeName
	return value, nil
}

func (r *Reader) Struct(path string) (map[string]any, error) {
	structType, err := r.FString()
	if err != nil {
		return nil, err
	}
	structID, err := r.GUID()
	if err != nil {
		return nil, err
	}
	id, err := r.OptionalGUID()
	if err != nil {
		return nil, err
	}
	v, err := r.StructValue(structType, path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"struct_type": structType, "struct_id": structID, "id": id, "value": v}, nil
}

func (r *Reader) StructValue(structType string, path string) (any, error) {
	switch structType {
	case "Vector":
		return r.Vector()
	case "Quat":
		return r.Quat()
	case "Transform":
		return r.Transform()
	case "Rotator":
		return r.Vector()
	case "DateTime":
		return r.U64()
	case "Guid":
		return r.GUID()
	case "LinearColor":
		red, err := r.Float()
		if err != nil {
			return nil, err
		}
		green, err := r.Float()
		if err != nil {
			return nil, err
		}
		blue, err := r.Float()
		if err != nil {
			return nil, err
		}
		alpha, err := r.Float()
		if err != nil {
			return nil, err
		}
		return map[string]any{"r": red, "g": green, "b": blue, "a": alpha}, nil
	default:
		return r.PropertiesUntilEnd(path)
	}
}

func (r *Reader) Vector() (map[string]any, error) {
	x, err := r.Double()
	if err != nil {
		return nil, err
	}
	y, err := r.Double()
	if err != nil {
		return nil, err
	}
	z, err := r.Double()
	if err != nil {
		return nil, err
	}
	return map[string]any{"x": x, "y": y, "z": z}, nil
}

func (r *Reader) Quat() (map[string]any, error) {
	x, err := r.Double()
	if err != nil {
		return nil, err
	}
	y, err := r.Double()
	if err != nil {
		return nil, err
	}
	z, err := r.Double()
	if err != nil {
		return nil, err
	}
	w, err := r.Double()
	if err != nil {
		return nil, err
	}
	return map[string]any{"x": x, "y": y, "z": z, "w": w}, nil
}

func (r *Reader) Transform() (map[string]any, error) {
	rotation, err := r.Quat()
	if err != nil {
		return nil, err
	}
	translation, err := r.Vector()
	if err != nil {
		return nil, err
	}
	scale, err := r.Vector()
	if err != nil {
		return nil, err
	}
	return map[string]any{"rotation": rotation, "translation": translation, "scale3d": scale}, nil
}

func (r *Reader) ArrayProperty(arrayType string, size int64, path string) (map[string]any, error) {
	count, err := r.U32()
	if err != nil {
		return nil, err
	}
	if count > 10_000_000 {
		return nil, fmt.Errorf("array count is unreasonable: %d", count)
	}
	if arrayType == "StructProperty" {
		propName, err := r.FString()
		if err != nil {
			return nil, err
		}
		propType, err := r.FString()
		if err != nil {
			return nil, err
		}
		if _, err := r.U64(); err != nil {
			return nil, err
		}
		typeName, err := r.FString()
		if err != nil {
			return nil, err
		}
		id, err := r.GUID()
		if err != nil {
			return nil, err
		}
		if err := r.Skip(1); err != nil {
			return nil, err
		}
		values := make([]any, 0, count)
		for range count {
			v, err := r.StructValue(typeName, path+"."+propName)
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		return map[string]any{"prop_name": propName, "prop_type": propType, "type_name": typeName, "id": id, "values": values}, nil
	}
	values, err := r.ArrayValue(arrayType, count, size, path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"values": values}, nil
}

func (r *Reader) ArrayValue(arrayType string, count uint32, size int64, path string) ([]any, error) {
	if arrayType == "ByteProperty" && size == int64(count) {
		values := make([]any, 0, count)
		for range count {
			b, err := r.Byte()
			if err != nil {
				return nil, err
			}
			values = append(values, int(b))
		}
		return values, nil
	}
	values := make([]any, 0, count)
	for range count {
		var v any
		var err error
		switch arrayType {
		case "EnumProperty", "NameProperty", "StrProperty":
			v, err = r.FString()
		case "Guid":
			v, err = r.GUID()
		case "IntProperty":
			var n int32
			n, err = r.I32()
			v = int(n)
		case "UInt32Property":
			var n uint32
			n, err = r.U32()
			v = uint(n)
		case "Int64Property":
			v, err = r.I64()
		case "FloatProperty":
			v, err = r.Float()
		case "BoolProperty":
			v, err = r.Bool()
		default:
			return nil, fmt.Errorf("unknown array type %q at %s", arrayType, path)
		}
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

func (r *Reader) MapProperty(path string) (map[string]any, error) {
	keyType, err := r.FString()
	if err != nil {
		return nil, err
	}
	valueType, err := r.FString()
	if err != nil {
		return nil, err
	}
	id, err := r.OptionalGUID()
	if err != nil {
		return nil, err
	}
	if _, err := r.U32(); err != nil {
		return nil, err
	}
	count, err := r.U32()
	if err != nil {
		return nil, err
	}
	if count > 10_000_000 {
		return nil, fmt.Errorf("map count is unreasonable: %d", count)
	}
	keyStructType := ""
	valueStructType := ""
	if keyType == "StructProperty" {
		keyStructType = hintOr(path+".Key", "Guid")
	}
	if valueType == "StructProperty" {
		valueStructType = hintOr(path+".Value", "StructProperty")
	}
	values := make([]any, 0, count)
	for range count {
		key, err := r.PropValue(keyType, keyStructType, path+".Key")
		if err != nil {
			return nil, err
		}
		value, err := r.PropValue(valueType, valueStructType, path+".Value")
		if err != nil {
			return nil, err
		}
		values = append(values, map[string]any{"key": key, "value": value})
	}
	return map[string]any{
		"key_type":          keyType,
		"value_type":        valueType,
		"key_struct_type":   keyStructType,
		"value_struct_type": valueStructType,
		"id":                id,
		"value":             values,
	}, nil
}

func (r *Reader) PropValue(typeName string, structTypeName string, path string) (any, error) {
	switch typeName {
	case "StructProperty":
		return r.StructValue(structTypeName, path)
	case "EnumProperty", "NameProperty", "StrProperty":
		return r.FString()
	case "IntProperty":
		v, err := r.I32()
		return int(v), err
	case "UInt32Property":
		v, err := r.U32()
		return uint(v), err
	case "Int64Property":
		return r.I64()
	case "BoolProperty":
		return r.Bool()
	case "Guid":
		return r.GUID()
	default:
		return nil, fmt.Errorf("unknown property value type %q at %s", typeName, path)
	}
}

func hintOr(path, fallback string) string {
	if v, ok := typeHints[path]; ok {
		return v
	}
	return fallback
}
