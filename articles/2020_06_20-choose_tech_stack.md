# How to choose a tech stack?

As time goes by, you are an experienced programmer, so you may choose a technical stack for your team, but, which tech
stack should you choose? Or, how should we choose tech stack?

## What's your Scenario?

The most important step, is figure out what's your scenario. What do you need?

- High concurrency? 
- Flexibility?
- Stability?
- New Features?

Only when you figure out what you need, can you choose a tech stack.

## Ecosystem Matters

Time can bring us lots of things, for software, it brings ecosystem. So what is ecosystem fot software? It's a collection
of softwares, with something important in the kernel, and lots of other software around it, for example, an OS is a
kernel(e.g. The Linux Kernel) and lots of utilities software(e.g. The GNU Software), kernel and utilities make up an
ecosystem.

So when we choose a framework, we have to check if it has a good ecosystem, for example, when we want to choose a Golang
web framework for our team, which one should we choose?

- Gin
- Echo
- Beego
- Others

When I check Github for stars, followers, contributors...etc, I found that GIN has the best ecosystem for:

- GIN has 39.2k stars, more than others
- GIN has 266 contributors, more than echo but less than Beego
- GIN has lots of utilities in https://github.com/gin-gonic/contrib
- GIN has less open issues than beego, but more than Echo

All these metrics shows that, GIN and Echo may be the better choice.

## Active Development Or Not

If a framework is abandoned by the author, then we should not choose it, because it will consume lots of time, we should
choose the active developing one. How can we check if a library/framework/utilities is still active developing?

- check README, if it is abandoned, the author may show it in README, or the web page for the project, just read it.
- check commit history, if the software still have commitments recently?
- check contributors, if a library that only have a few contributors, it may be slow to response or development

But, we **should** choose a stable branch for production use, active development means that there may be some bugs
that haven't been discovered yet, just **use the stable branch of a active developing software**.

## A Good Community

Is there a friendly community? it matters. Because if the community is active, lots of pitfalls had already been
discovered by others, so if we want to find a solution, just check the docs, or StackOverflow! Google will help us.

But if you choose a fresh, newborn software, well, Good luck :)

## Conclution

It is very important for us to choose a good tech stack, because switch tech stacks need a lots of time, and all
expirence you got(with the previous tech stack) will be thrown to the trash.

Which one do you choose?

- FreeBSD or Linux?
- Java, Golang, or Python?
- GIN, Echo or something else?

---

Refs:

- https://en.wikipedia.org/wiki/Software_ecosystem#:~:text=In%20the%20context%20of%20software,technical%20(the%20Ruby%20ecosystem).
