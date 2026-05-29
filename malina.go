// malina lets you write Go applications that directly integrate stable-diffusion.cpp
// (https://github.com/leejet/stable-diffusion.cpp) for fully local text-to-image and
// image editing using hardware acceleration.
//
//   - Run any stable-diffusion.cpp supported model on Linux, macOS, or Windows.
//   - Use any available hardware acceleration such as CUDA (https://en.wikipedia.org/wiki/CUDA),
//     Metal (https://en.wikipedia.org/wiki/Metal_(API)), or Vulkan (https://en.wikipedia.org/wiki/Vulkan)
//     for maximum performance.
//   - malina uses the purego (https://github.com/ebitengine/purego) and ffi (https://github.com/JupiterRider/ffi)
//     packages so CGo is not needed.
//   - Works with the newest stable-diffusion.cpp releases so you can use the latest features and
//     model support.
//
// malina is the text-to-image sibling of bucky (https://github.com/ardanlabs/bucky), which
// provides the same kind of FFI bindings for whisper.cpp.
package main
