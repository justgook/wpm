var x: u32 = 46;

export fn next() f32 {
    x +%= 0x9E3779B9;
    var z: u32 = x;

    z ^= z >> 16;
    z *%= 0x21f0aaad;
    z ^= z >> 15;
    z *%= 0x735a2d97;
    z ^= z >> 15;

    // Shift to 23 bits and normalize
    return @as(f32, @floatFromInt(z >> 9)) * 0x1.0p-23;
}
