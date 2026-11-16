package sd

import (
	"fmt"
	"strings"
)

const rngNames = "std_default,cuda,cpu"
const sampleNames = "euler,euler_a,heun,dpm2,dpm++2s_a,dpm++2m,dpm++2mv2,ipndm,ipndm_v,lcm,ddim_trailing,tcd,res_multistep,res_2s,er_sde,euler_cfg_pp,euler_a_cfg_pp,euler_ge,dpm++2m_sde,dpm++2m_sde_bt,lms"
const schedulerNames = "discrete,karras,exponential,ays,gits,sgm_uniform,simple,smoothstep,kl_optimal,lcm,bong_tangent,ltx2,logit_normal,flux2,flux,beta,llada_image"
const predictionNames = "eps,v,edm_v,sd3_flow,flux_flow,sefi_flow,minit2i_flow,sensenova_u1_flow,flux3_flow,h3_av_flow,ming_image_flow"
const previewNames = "none,proj,tae,vae"
const loraNames = "auto,immediately,at_runtime"
const hiresNames = "none,latent,latent_nearest,latent_nearest_exact,latent_antialiased,latent_bicubic,latent_bicubic_antialiased,lanczos,nearest,model"
const controlNames = "none,openpose,depth,scribble,canny,normal,segment,tile,repaint"
const cacheNames = "disabled,easycache,ucache,dbcache,taylorseer,cache_dit,spectrum"

func enumName(value int32, names string, auto bool) (string, error) {
	values := strings.Split(names, ",")
	if auto && int(value) == len(values) {
		return "auto", nil
	}
	if value < 0 || int(value) >= len(values) {
		return "", fmt.Errorf("invalid enum value %d", value)
	}
	return values[value], nil
}

func parseEnum(value, names string) (int32, error) {
	if strings.ContainsRune(value, 0) {
		return 0, fmt.Errorf("enum name contains NUL")
	}
	values := strings.Split(names, ",")
	for i, name := range values {
		if name == value {
			return int32(i), nil
		}
	}
	return int32(len(values)), nil
}

func enumOption(value any) (string, bool, error) {
	var text string
	var err error
	switch v := value.(type) {
	case SDType:
		if v == SDTypeCount {
			text = "auto"
		} else {
			text, err = SDTypeName(v)
		}
	case RngType:
		text, err = enumName(int32(v), rngNames, true)
	case SampleMethod:
		text, err = enumName(int32(v), sampleNames, true)
	case Scheduler:
		text, err = enumName(int32(v), schedulerNames, true)
	case Prediction:
		text, err = enumName(int32(v), predictionNames, true)
	case LoraApplyMode:
		text, err = enumName(int32(v), loraNames, false)
	case HiresUpscaler:
		text, err = enumName(int32(v), hiresNames, false)
	case ControlType:
		text, err = enumName(int32(v), controlNames, false)
	case CacheMode:
		text, err = enumName(int32(v), cacheNames, false)
	case SDVaeFormat:
		text, err = enumName(int32(v)+1, "auto,flux,sd3,flux2,wan", false)
	default:
		return "", false, nil
	}
	return text, true, err
}
