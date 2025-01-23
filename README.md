# cli-llm
for personal usage; command line LLM script
Usage: llm [OPTIONS] [PROMPT]

Options:
  -p, --prompt TEXT               Input prompt for deepseek.
  -n, --no-stream                  the streaming of the
                                  response.(totally faster but need to wait
                                  until all results are generated),
                                  model-R does not support
                                  stream!
  -m, --model [coder|chat|creative|coder-R|chat-R|creative-R]
                                  Choose the model to use.default is
                                  coder. the deepseek-coder is not recommanded
                                  in CLI, Now DeepSeek-R1 Reansoner is
                                  available, append an `-R` to use the
                                  reasoner of current mode, e.g. `coder-R`,
                                  `chat-R`, `creative-R`
  -o, --output-codes TEXT         Output the code to the target
                                  file.Only one code block is
                                  supported(IN PROGESS)
  --help                          Show this message and exit.
