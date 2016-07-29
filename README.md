
gcslock is a scalable, distributed mutex that can be used to serialize computations anywhere on the global internet. (Disclaimer: The author works for Gogole but this is not an official Google product.)

# What is this?

Once upon a time, in CS grad school, I was given an interesting homework assignment: using only native Unix shell commands (this was the pre-Linux era), develop a mutual exclusion mechanism (aka, a [mutex](https://en.wikipedia.org/wiki/Mutual_exclusion)) to serialize blocks of commands in a shell script. Before I reveal the solution, I'd suggest spending some time thinking about how you might solve this problem.

The trick was to make use of the fact that file system links can be created only once. Link creation requests have precisely the atomic "test & set" semantics required to implement a reliable mutex. If the link doesn't already exist, it's created, atomically, in kernel space (so no other process can sneak in and create the link while your request is in progress. And if the link already exists, the request fails. So, a reliable mutex lock can be implemented by looping on the link creation request until it succeeds, then doing the computation you want to protect, and finally removing the link when you're done.

The approach described above works quite well but it also has a few problems:

1. It's inefficient because it entails relatively expensive and slow file system operations.
2. It's unreliabile: consider what happens if a script holding the lock terminates before it removes the link.

Of course, this approach was meant for a toy mutex, not something to be used in the real world. Modern kernels offer a multitude of more efficient and more resilient mutex and related synchronization primitives. But even these mechanisms share a major limitation with the file linking technique: their scope is limited to a single operating system image.

Fast forward to more modern times. I faced a design problem that required serializing distributed computations across multiple servers running in the cloud. There are several well known techniques, like using Redis or a distributed database with transaction semantics. But all of the approaches I found seem too complex, and require me to too much code. Remembering the simplicity of my homework assignment from many years ago, I wondered whether that approach could be extended to the cloud computing era, whether atomic file creation in the cloud could be used to build a planet-wide distributed mutex.

I dismissed Amazon's S3 an an implementation choice because it offers only eventual consistency, meaning that object changes made at one point in time are not guaranteed to be seen by other processes until some (typically short) period of time in the future. Obviously this will not work for a mutex, which requires all contending processes to see precisely the same state at the same time. Otherwise, two processes contending for a lock could easily find themselves both believing they had successfully acquired it.

Google Cloud Storage implements read-after-write consistency, meaning when a change is written, any process that reads that object, anywhere on the planet, is guantanteed to see the change. In other words, GCS offers an integrity model that works, at global scope, very similary to the way a local file system works, where we take for granted that changes to a file are guaranteed to be seen, immediately after the change, by all subsequent readers.

But how to implement "create once" semantics? If contending processes attempt to create a lock file in the cloud, won't they simply overwrite each other's objects? Well, thanks to another GCS feature, [object versioning](https://cloud.google.com/storage/docs/object-versioning), we can request precisely the semantics we need:

> If you set the x-goog-if-generation-match header to 0 when uploading an object, Cloud Storage only performs the specified request if the object does not currently exist. For example, you can perform a PUT request to create a new object with x-goog-if-generation-match set to 0, and the object will be created only if it doesn't already exist. Otherwise, Google Cloud Storage aborts the update with a status code of 412 Precondition Failed.

This is exactly the sort of guarantee provided by the kernel when we attempt to create a file system link. The combination of read-after-write consistency and atomic create-once semantics gives us everything we need to build a distributed mutex that can be used to serialize computations anywhere across the internet.

# Ok, so how do I use it?

The reference implementation in this repo is written in Go. To use gcslock,
do the following:

1. Obtain the library via 'go get github.com/marcacohen/gcsmutex'.
1. 
