#!/usr/bin/env python3
"""Compile the real Swift bridge with mock work and run under ThreadSanitizer."""

from pathlib import Path
import platform
import subprocess
import tempfile


def main():
    directory = Path(__file__).resolve().parent
    with tempfile.TemporaryDirectory(prefix="integterm-storekit-test-") as temporary:
        output = Path(temporary)
        source = output / "BridgeTests.swift"
        source.write_text(
            (directory / "StoreKit2Bridge.swift").read_text()
            + "\n"
            + (directory / "StoreKit2BridgeTests.swift").read_text()
        )
        binary = output / "bridge-tests"
        subprocess.run(
            [
                "xcrun", "swiftc", "-parse-as-library", "-sanitize=thread",
                "-target", platform.machine() + "-apple-macos13.0",
                str(source), "-o", str(binary),
            ],
            check=True,
            timeout=60,
        )
        subprocess.run([str(binary)], check=True, timeout=20)


if __name__ == "__main__":
    main()
