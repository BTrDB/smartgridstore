package upmuparser

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Upmu_vector struct {
	Phase_in_degrees float32
	Fundamental_magnitude_volts float32
} // 8 bytes

type Upmu_one_second_set struct {
	Sample_interval_in_milliseconds float32
	Timestamp [6]int32
	Status [120]int32
	L1_e_vector_space [120]Upmu_vector
	L2_e_vector_space [120]Upmu_vector
	L3_e_vector_space [120]Upmu_vector
	C1_e_vector_space [120]Upmu_vector
	C2_e_vector_space [120]Upmu_vector
	C3_e_vector_space [120]Upmu_vector
} // 6268 bytes

type Upmu_one_second_expansion_set_one struct {
	Fundamental_watts_total [120]float32
	Fundamental_var_total [120]float32
	Fundamental_va_total [120]float32
	Fundamental_dpf_total [120]float32
	Frequency_l1_e_one_second [120]float32
	Frequency_l1_e_c37 [120]float32
} // 2880 bytes

type Upmu_one_second_output_standard struct {
	Data Upmu_one_second_set
	Upmu_debug_info_pll [4]uint32 // the actual struct definition says 'umpu'
	Upmu_debug_info_gps [7]float32
} // 6312 bytes

type Upmu_one_second_output_expansion_set_one struct {
    Basic_data Upmu_one_second_output_standard
    Expansion_set_one Upmu_one_second_expansion_set_one
} // 9192 bytes

const UPMU_ONE_SECOND_OUTPUT_STANDARD_SIZE int = 6312
const UPMU_ONE_SECOND_EXPANSION_SET_ONE_SIZE int = 2880

type Sync_Output_Type int

const (
	OUTPUT_STANDARD Sync_Output_Type = iota
	EXPANSION_SET_ONE
)

type Sync_Output struct {
	Version Sync_Output_Type
	Data Upmu_one_second_output_expansion_set_one
}

type decoder struct {
	index int
	data []uint8
}

func (d *decoder) Read(b []byte) (n int, err error) {
	var i int
	n = len(b)
	var outOfSpace bool = false
	if len(d.data) - d.index < n {
		n = len(d.data) - d.index
		outOfSpace = true
	}
	for i = 0; i < n; i++ {
		b[i] = d.data[d.index + i]
	}
	d.index = d.index + n
	if outOfSpace {
		err = io.EOF
	}
	return
}

func (d *decoder) finished() bool {
	return d.index == len(d.data)
}

func parse_sync_output(d *decoder) (*Sync_Output, error) {
	var output *Sync_Output = &Sync_Output{}
	var newversion bool = false

	err := binary.Read(d, binary.LittleEndian, &output.Data.Basic_data)
	if err != nil {
		fmt.Printf("Error parsing sync_output (basic data): %v\n", err)
		return nil, err
	}

	for _, status := range output.Data.Basic_data.Data.Status {
		if status & 0xe0 != 0 {
			newversion = true
			break
		}
	}

	if newversion {
		output.Version = EXPANSION_SET_ONE
		err = binary.Read(d, binary.LittleEndian, &output.Data.Expansion_set_one)
		if err != nil {
			fmt.Printf("Error parsing sync_output (expansion set one): %v\n", err)
			return nil, err
		}
	} else {
		output.Version = OUTPUT_STANDARD
	}

	return output, nil
}

func ParseSyncOutArray(data []byte) ([]*Sync_Output, error) {
	var dataLen int = len(data)
	var numSyncOutputs int = dataLen / UPMU_ONE_SECOND_OUTPUT_STANDARD_SIZE

	var dec *decoder = &decoder{index: 0, data: data}
	var output *Sync_Output
	var outputs = make([]*Sync_Output, 0, numSyncOutputs)
	var err error = nil

	for !dec.finished() && err == nil {
		output, err = parse_sync_output(dec)
		outputs = append(outputs, output)
	}

	return outputs, err
}

/* These are functions to use for getting values for streams. It's not concise,
   but it's needed since the data is parsed to structs, which as far as I know
   can't be subscripted with strings. */

func GetL1Mag(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.L1_e_vector_space[index].Fundamental_magnitude_volts)
}

func GetL1Ang(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.L1_e_vector_space[index].Phase_in_degrees)
}

func GetL2Mag(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.L2_e_vector_space[index].Fundamental_magnitude_volts)
}

func GetL2Ang(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.L2_e_vector_space[index].Phase_in_degrees)
}

func GetL3Mag(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.L3_e_vector_space[index].Fundamental_magnitude_volts)
}

func GetL3Ang(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.L3_e_vector_space[index].Phase_in_degrees)
}

func GetC1Mag(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.C1_e_vector_space[index].Fundamental_magnitude_volts)
}

func GetC1Ang(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.C1_e_vector_space[index].Phase_in_degrees)
}

func GetC2Mag(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.C2_e_vector_space[index].Fundamental_magnitude_volts)
}

func GetC2Ang(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.C2_e_vector_space[index].Phase_in_degrees)
}

func GetC3Mag(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.C3_e_vector_space[index].Fundamental_magnitude_volts)
}

func GetC3Ang(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.C3_e_vector_space[index].Phase_in_degrees)
}

func GetLockState(index int, obj *Sync_Output) float64 {
	return float64(obj.Data.Basic_data.Data.Status[index])
}

func GetFundW(index int, obj *Sync_Output) float64 {
	if (obj.Version < EXPANSION_SET_ONE) {
		panic("invalid type for insert getter")
	}
	return float64(obj.Data.Expansion_set_one.Fundamental_watts_total[index])
}

func GetFundVar(index int, obj *Sync_Output) float64 {
	if (obj.Version < EXPANSION_SET_ONE) {
		panic("invalid type for insert getter")
	}
	return float64(obj.Data.Expansion_set_one.Fundamental_var_total[index])
}

func GetFundVA(index int, obj *Sync_Output) float64 {
	if (obj.Version < EXPANSION_SET_ONE) {
		panic("invalid type for insert getter")
	}
	return float64(obj.Data.Expansion_set_one.Fundamental_va_total[index])
}

func GetFundDPF(index int, obj *Sync_Output) float64 {
	if (obj.Version < EXPANSION_SET_ONE) {
		panic("invalid type for insert getter")
	}
	return float64(obj.Data.Expansion_set_one.Fundamental_dpf_total[index])
}

func GetFreqL11S(index int, obj *Sync_Output) float64 {
	if (obj.Version < EXPANSION_SET_ONE) {
		panic("invalid type for insert getter")
	}
	return float64(obj.Data.Expansion_set_one.Frequency_l1_e_one_second[index])
}

func GetFreqL1C37(index int, obj *Sync_Output) float64 {
	if (obj.Version < EXPANSION_SET_ONE) {
		panic("invalid type for insert getter")
	}
	return float64(obj.Data.Expansion_set_one.Frequency_l1_e_c37[index])
}

type InsertGetter func(int, *Sync_Output) float64

var STREAMS [19]string = [19]string{"L1MAG", "L1ANG", "L2MAG", "L2ANG", "L3MAG", "L3ANG","C1MAG", "C1ANG", "C2MAG", "C2ANG", "C3MAG", "C3ANG", "LSTATE", "FUND_W", "FUND_VAR", "FUND_VA", "FUND_DPF", "FREQ_L1_1S", "FREQ_L1_C37"}

func (s *Sync_Output) GetInsertGetters() []InsertGetter {
	var getters = make([]InsertGetter, 0, len(STREAMS))
	if (s.Version >= OUTPUT_STANDARD) {
		getters = append(getters, GetL1Mag, GetL1Ang, GetL2Mag, GetL2Ang, GetL3Mag, GetL3Ang, GetC1Mag, GetC1Ang, GetC2Mag, GetC2Ang, GetC3Mag, GetC3Ang, GetLockState)
	}
	if (s.Version >= EXPANSION_SET_ONE) {
		getters = append(getters, GetFundW, GetFundVar, GetFundVA, GetFundDPF, GetFreqL11S, GetFreqL1C37)
	}
	return getters
}

func (s *Sync_Output) SampleRate() float32 {
	return s.Data.Basic_data.Data.Sample_interval_in_milliseconds
}

func (s *Sync_Output) Times() [6]int32 {
	return s.Data.Basic_data.Data.Timestamp
}
