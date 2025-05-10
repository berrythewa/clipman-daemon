Darwin vs. Linux Clipboard Implementation: Feature Checklist
1. Supported Content Types
Linux: Text, Image, HTML, RTF, File Path, File List, URL, Custom MIME types.
Darwin (Current): Text, Image (via zyedidia/clipboard), basic change detection.
Gaps:
No support for HTML, RTF, File Path, File List, URL, or custom MIME types.
No content type detection beyond text/image.
---
2. Change Detection
Linux: Uses content hash, X11/Wayland events, and adaptive polling.
Darwin: Uses NSPasteboard change count (but currently a stub).
Gaps:
getChangeCount() is a stub; needs real CGO/Objective-C implementation.
No adaptive polling or event-based optimizations.
---
3. Content Caching
Linux: In-memory cache with expiry, hash-based change detection, defensive copying.
Darwin: No cache; always reads from system clipboard.
Gaps:
No in-memory cache for recent clipboard content.
No hash-based change detection.
---
4. Content Type Detection
Linux: Detects type (text, image, HTML, RTF, file, URL) using content and format.
Darwin: No detection; just returns text or image.
Gaps:
No detection for URLs, file paths, HTML, RTF, etc.
---
5. Custom MIME Type Support
Linux: Extensible via handler registration.
Darwin: None.
Gaps:
No extensibility for custom types.
---
6. Clipboard Write Support
Linux: Can write all supported types.
Darwin: Only writes text.
Gaps:
No support for writing images, HTML, RTF, file paths, or file lists.
---
7. Logging and Error Handling
Linux: Structured logging, robust error handling, retry logic.
Darwin: Minimal error handling, no logging interface.
Gaps:
No logging interface.
No retry logic for clipboard operations.
---
8. Thread Safety
Linux: Mutexes for cache and content.
Darwin: No explicit thread safety.
Gaps:
Needs mutexes for cache/content if caching is added.
---
9. Adaptive Polling
Linux: Polling interval adapts to activity.
Darwin: Fixed interval.
Gaps:
No adaptive polling.
---
10. API/Interface Parity
Linux: Implements all methods in the Clipboard interface.
Darwin: Implements required methods, but with limited functionality.
Gaps:
Needs to match Linux in method completeness and extensibility.
---
Summary Table
| Feature | Linux | Darwin (Current) | Darwin (Needed) |
|------------------------|--------------|------------------|-----------------|
| Text | ✔️ | ✔️ | ✔️ |
| Image | ✔️ | ✔️ | ✔️ |
| HTML | ✔️ | ❌ | ✔️ |
| RTF | ✔️ | ❌ | ✔️ |
| File Path/List | ✔️ | ❌ | ✔️ |
| URL Detection | ✔️ | ❌ | ✔️ |
| Custom MIME Types | ✔️ | ❌ | ✔️ |
| Change Detection | ✔️ | (Stub) | ✔️ |
| In-Memory Cache | ✔️ | ❌ | ✔️ |
| Defensive Copying | ✔️ | ❌ | ✔️ |
| Logging | ✔️ | ❌ | ✔️ |
| Adaptive Polling | ✔️ | ❌ | ✔️ |
| Retry Logic | ✔️ | ❌ | ✔️ |
| Thread Safety | ✔️ | ❌ | ✔️ |
---
Next Steps
To bring Darwin up to Linux parity, you need:
    1. Real getChangeCount() using CGO/Objective-C.
    2. Support for HTML, RTF, file paths/lists, and URL detection.
    3. In-memory cache with defensive copying.
    4. Content type detection logic.
    5. Logging and error handling.
    6. Adaptive polling.
    7. Thread safety (mutexes).
    8. Extensible custom MIME type support