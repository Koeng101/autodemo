# autodemo

autodemo is an demo of a cloud lab + lab computing environment + LLM integration built by Keoni Gandall. I mean this to demonstrate some ideas I have about how biological cloud labs should work in the future.

[![Demo of Autodemo](https://img.youtube.com/vi/BEUKxsUfO9w/0.jpg)](https://youtu.be/BEUKxsUfO9w)

[Gist of full repo code to copy paste into an LLM to ask questions about it](https://gist.github.com/Koeng101/17a43a6e1511522e888fb02fdf050ba6)

The core concepts are fairly simple:
1. Have a cloud lab that can be used with JSON, or a simple standard interface
2. Add a lua environment on top of that cloud lab to enable interactivity and make programming simple
3. Have a large language model create that lua for users
4. Create a standard library in lua for executing different experiments

The interactivity is key: there are some many places that you should be doing checks during protocols to make sure everything is working. Just having protocols described as static JSON is insufficient for programming complicated biological protocols. Using something like python is also difficult - it isn't built to be a scripting language within a larger system, and that becomes very apparent once you try to build this with that. Lua is perfect for scripting in this small, constrained environment.

Most biologists aren't coders. That is fine, now that we have LLMs, but those LLMs need a lot of training to begin working well for target environments. Unlike previous cloud labs which primarily target expensive customers (>$40k spend a month), the one that will win in the future will be the one that makes commodity, everyday protocols workable. There is an untapped positive feedback loop here - the more the cloud lab is used, the better its systems will become. For the business minded out there, this is also a fantastic moat, because that positive feedback loop will only be for your particular lab environment.

# Long term architecture
```
1. Standard Library: libB (Lua)
2. lua execution layer
3. Script Layer (JSON)

A. External libraries
B. Reasoning
C. Tools
D. Lab Inventory (State)
E. Batch interface
```

libB (name of my current lua library) + the lua execution layer enables biological protocols to be expressed as code - importantly, not only linear, end to end protocols (expressed as JSON, or as simple python code) - but dynamic programs that can change based off of lab input, all without leaving the execution environment's servers. All interactions happen over JSON, and if a user doesn't want to use our dynamic execution environment, they can always set up their own by only using the scripting layer + webhooks.

The Standard Library, libB, implements many useful programs. It is written in lua to enable localized scripting. Not only does it run remotely in the target lab, but you can also use its functions for local scripting, meaning you can port all programs written in libB to your favorite language - Python, Go, Rust, Zig, etc. The business logic will all be expressed consistently due to the standardization of the lua5.1 computing environment.

### External Libraries
Any code written to create protocols can be easily imported and shared, as it is all just lua. Since the execution environment is dynamic, all these protocols can interact and build on each other, all while maintaining their own internal QA/QC business logic - even if this business logic can take days or weeks to execute.

### Reasoning
We try to centralize code executing on the cloud lab so that we can reason over it automatically with models. In the beginning, this may be fruitless and frustrating, but as more examples are created and troubleshot, one gains invaluable experience in how to actually reason about doing biological experiments in our lab. Eventually, this enables the model to reason about failures and use tools to try to fix its own failures. This finetuning data will be created during the process of creating libB, but can be continuously built upon during every experiment.

The endpoint of this is a system which can autonomously create its own protocols, troubleshoot them both on the physical execution level and the biological level, and answer questions about the biological world without having biologists having to learn to program or having to learn how to use a pipette. However, this only is valuable with enough data to finetune the models - unlike every cloud lab right now, one must create commodity and everyday protocols and use them to create the necessary data.

### Tools
Since everything is executing in a lua scripting environment, we can arbitrarily increase capabilities. Rather than using clunky JSON tools, we can integrate our tools as code. Need a uniprot API? Just add `libB.uniprot`. Need search? Add `libB.search`. In particular, I think it will be important to have tools to do bulk compute over sequencing data.

There are some things that cannot be expressed well in lua though, so we can always reach for `<python></python>` rather than `<lua></lua>`. You can imagine models using your favorite data analysis tools on incoming data from a biological experiment to make reasoned guesses on what is happening or what to do next.

### Lab Inventory
Lab inventory becomes rather difficult, but also an opportunity. The inventory *must* be physically in the lab, which creates a great opportunity for cloud labs to make money off of both reagents and robot time. Just don't be like transcriptic (used to): don't make your users ship their own enzymes to your cloud lab. Defeats the point of having a cloud lab.

### Batch interface
One of the most important things that libB can do is create a batch API interface. Many protocols will have economic scaling laws: for example, oligo pool assembly only becomes a good idea if you are assembling >$6000 of DNA (a little over the cost of an oligo pool). libB will allow users to seamlessly call batched APIs, and get the same convenient status updates, which lets you take a chunk of larger protocols. This makes many protocols economically efficient, whereas in any other case they wouldn't even be possible. 
