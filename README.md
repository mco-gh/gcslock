
gcslock is a scalable, distributed mutex that can be used to 
serialize computations anywhere on the global internet.

(Disclaimer: The author works for Gogole but this is not an official Google product.)

# What is this?

Once upon a time, in CS grad school, I was given an interesting homework 
assignment: using only Unix shell commands (this was the pre-Linux era),
develop a mutual exclusion mechanism (aka, a "mutex") to serialize
blocks of shell commands. Before I reveal the solution, I'd suggest spending
some time thinking about how you might solve this problem.

The trick was to make use of the fact that file system links can be created
only once. Link creation requests have precisely the atomic "test & set" 
semantics required to implement a reliable mutex. If the link doesn't already 
exist, it's created, atomically, in kernel space, so no other process can
sneak in and create the link while your request is in progress. And if the
link already exists, the request fails. So, mutex lock can be implemented
by looping on the link creation request, until it succeeds, then doing the
computation you want to protect, then removing the link. 


The approach described above works quite well but it's very inefficient because
it entails file system operations. It also has reliability issues: what happens 
if a script holding the lock terminates before it removes the link? For these
and other reasons, this was just a toy mutex, not something to be used in the 
real world. The pthreads mutex object is the right tools for serious use but
both versions suffer from a signicant limitation: their scope is limited to 
a single operating system image.

Fast forward to more modern times. I faced a design problem that required 
serializing distributed computations across multiple servers. After a bit
of searching, I found several options, like using Redis or a distributed 
database with transactions. But I remembered this homework assignment from
long ago and I wondered whether that approach could be extended, whether
atomic file creation in the cloud could be used to build a distributed 
mutex in the cloud.

I dismissed S3 because it offers only eventually consistency, meaning that
object changes made at one point in time are not guaranteed to be seen by 
other processes until some (typically short) period of time in the future. 
Obviously this will not work for a mutex, which requires all contending
processes to see the same state at the same time.

Google Cloud Storage implements read-after-write consistency, meaning
when a change is written, any process that reads that object, anywhere
on the planet, is guantanteed to see the change. In other words, GCS
offers an integrity model that works, at global scope, just as a local
file system does, where we take for granted that changes to a file are
guaranteed to be seen by subsequent readers.

But how to implement "create once" semantics? If contending processes
attempt to create a lock file in the cloud, won't they simply overwrite 
each other's objects? Well, thanks to another GCS feature, object version,
we can request precisely the semantics we need:

If you set the x-goog-if-generation-match header to 0 when uploading an object, Cloud Storage only performs the specified request if the object does not currently exist. For example, you can perform a PUT request to create a new object with x-goog-if-generation-match set to 0, and the object will be created only if it doesn't already exist. Otherwise, Google Cloud Storage aborts the update with a status code of 412 Precondition Failed.

This is exactly the sort of guarantee provided by the kernel when we attempt 
to create a file system link.

The combination of read-after-write consistency and atomic create-once semantics
give us everything we need to build a distributed mutex that can be used to 
serialize computations anywhere across the internet.

# How do I use it?

The reference implementation in this repo is written in Go. To use gcslock,
do the following:

1. Obtain the library via 'go get github.com/marcacohen/gcsmutex'.
1. 
