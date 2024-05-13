// Copyright (c) 2024 Karl Gaissmaier
// SPDX-License-Identifier: MIT

package bart

// Please read the ART paper ./doc/artlookup.pdf
// to understand the baseIndex algorithm.

// hostMasks as lookup table
var hostMasks = []uint8{
	0b1111_1111, // bits == 0
	0b0111_1111, // bits == 1
	0b0011_1111, // bits == 2
	0b0001_1111, // bits == 3
	0b0000_1111, // bits == 4
	0b0000_0111, // bits == 5
	0b0000_0011, // bits == 6
	0b0000_0001, // bits == 7
	0b0000_0000, // bits == 8
}

const (

	// baseIndex of the first host route: prefixToBaseIndex(0,8)
	firstHostIndex = 0b1_0000_0000 // 256

	// baseIndex of the last host route: prefixToBaseIndex(255,8)
	lastHostIndex = 0b1_1111_1111 // 511
)

// allot, bart is a balanced-art, bart has no allotment table for each stride table.
// bart uses popcount slice compression with no fixed-size allotment arrays.
// For some algorithms that would be either too complex or too slow without
// an allotment table we build it on the fly.
//
// It's a mix of iteration and rec-descent to make it fast.
// Keep it fast, input validation must be done on calling site.
//
//nolint:unused
func allot(allotTbl *[maxNodePrefixes]bool, idx uint) {
	// microbenchmarking, recursion is faster than iteration for idx >= 4
	if idx >= 4 {
		allotRec(allotTbl, idx)
		return
	}

	// microbenchmarking, iteration is faster than recursion for idx <= 4
	allotTbl[idx] = true
	for idx < firstHostIndex {
		if allotTbl[idx] {
			// trick, the allotTbl itself is the stack
			allotTbl[idx<<1] = true
			allotTbl[idx<<1+1] = true
		}
		idx++
	}
}

// allotRec, recursive part of allot.
// Keep it fast, input validation must be done on calling site.
func allotRec(allotTbl *[maxNodePrefixes]bool, idx uint) {
	allotTbl[idx] = true
	// idx has reached last stage
	if idx >= firstHostIndex {
		return
	}

	allotRec(allotTbl, idx<<1)
	allotRec(allotTbl, idx<<1+1)
}

// prefixToBaseIndex, maps a prefix table as a 'complete binary tree'.
// This is the so-called baseIndex a.k.a heapFunc:
func prefixToBaseIndex(octet byte, prefixLen int) uint {
	return uint(octet>>(strideLen-prefixLen)) + (1 << prefixLen)
}

// octetToBaseIndex, just prefixToBaseIndex(octet, 8), a.k.a host routes
// but faster, use it for host routes in Lookup.
func octetToBaseIndex(octet byte) uint {
	return uint(octet) + firstHostIndex // just: octet + 256
}

// lowerUpperBound, get range of host routes for this prefix
//
//	prefix: 32/6
//	idx:    72
//	lower:  256 + 32 = 288
//	upper:  256 + (32 | 0b0000_0011) = 291
func lowerUpperBound(idx uint) (uint, uint) {
	octet, bits := baseIndexToPrefix(idx)
	return octetToBaseIndex(octet), octetToBaseIndex(octet | hostMasks[bits])
}

// baseIndexToPrefixMask, calc the bits from baseIndex and octect depth
func baseIndexToPrefixMask(baseIdx uint, depth int) int {
	_, pfxLen := baseIndexToPrefix(baseIdx)
	return depth*strideLen + pfxLen
}

// baseIndexToPrefix returns the octet and prefix len of baseIdx.
// It's the inverse to prefixToBaseIndex.
func baseIndexToPrefix(baseIdx uint) (octet byte, pfxLen int) {
	pfx := baseIdx2Pfx[baseIdx]
	return pfx.octet, pfx.bits
}

// baseIdx2Pfx, octet and CIDR bits of baseIdx as lookup table.
// Use the pre computed lookup table, bits.LeadingZeros is too slow.
//
//  func baseIndexToPrefix(baseIdx uint) (octet uint, pfxLen int) {
//  	nlz := bits.LeadingZeros(baseIdx)
//  	pfxLen = strconv.IntSize - nlz - 1
//  	octet = (baseIdx & (0xFF >> (8 - pfxLen))) << (8 - pfxLen)
//  	return octet, pfxLen
//  }

var baseIdx2Pfx = [512]struct {
	octet byte
	bits  int
}{
	{0, -1},  // idx ==   0 invalid!
	{0, 0},   // idx ==   1
	{0, 1},   // idx ==   2
	{128, 1}, // idx ==   3
	{0, 2},   // idx ==   4
	{64, 2},  // idx ==   5
	{128, 2}, // idx ==   6
	{192, 2}, // idx ==   7
	{0, 3},   // idx ==   8
	{32, 3},  // idx ==   9
	{64, 3},  // idx ==  10
	{96, 3},  // idx ==  11
	{128, 3}, // idx ==  12
	{160, 3}, // idx ==  13
	{192, 3}, // idx ==  14
	{224, 3}, // idx ==  15
	{0, 4},   // idx ==  16
	{16, 4},  // idx ==  17
	{32, 4},  // idx ==  18
	{48, 4},  // idx ==  19
	{64, 4},  // idx ==  20
	{80, 4},  // idx ==  21
	{96, 4},  // idx ==  22
	{112, 4}, // idx ==  23
	{128, 4}, // idx ==  24
	{144, 4}, // idx ==  25
	{160, 4}, // idx ==  26
	{176, 4}, // idx ==  27
	{192, 4}, // idx ==  28
	{208, 4}, // idx ==  29
	{224, 4}, // idx ==  30
	{240, 4}, // idx ==  31
	{0, 5},   // idx ==  32
	{8, 5},   // idx ==  33
	{16, 5},  // idx ==  34
	{24, 5},  // idx ==  35
	{32, 5},  // idx ==  36
	{40, 5},  // idx ==  37
	{48, 5},  // idx ==  38
	{56, 5},  // idx ==  39
	{64, 5},  // idx ==  40
	{72, 5},  // idx ==  41
	{80, 5},  // idx ==  42
	{88, 5},  // idx ==  43
	{96, 5},  // idx ==  44
	{104, 5}, // idx ==  45
	{112, 5}, // idx ==  46
	{120, 5}, // idx ==  47
	{128, 5}, // idx ==  48
	{136, 5}, // idx ==  49
	{144, 5}, // idx ==  50
	{152, 5}, // idx ==  51
	{160, 5}, // idx ==  52
	{168, 5}, // idx ==  53
	{176, 5}, // idx ==  54
	{184, 5}, // idx ==  55
	{192, 5}, // idx ==  56
	{200, 5}, // idx ==  57
	{208, 5}, // idx ==  58
	{216, 5}, // idx ==  59
	{224, 5}, // idx ==  60
	{232, 5}, // idx ==  61
	{240, 5}, // idx ==  62
	{248, 5}, // idx ==  63
	{0, 6},   // idx ==  64
	{4, 6},   // idx ==  65
	{8, 6},   // idx ==  66
	{12, 6},  // idx ==  67
	{16, 6},  // idx ==  68
	{20, 6},  // idx ==  69
	{24, 6},  // idx ==  70
	{28, 6},  // idx ==  71
	{32, 6},  // idx ==  72
	{36, 6},  // idx ==  73
	{40, 6},  // idx ==  74
	{44, 6},  // idx ==  75
	{48, 6},  // idx ==  76
	{52, 6},  // idx ==  77
	{56, 6},  // idx ==  78
	{60, 6},  // idx ==  79
	{64, 6},  // idx ==  80
	{68, 6},  // idx ==  81
	{72, 6},  // idx ==  82
	{76, 6},  // idx ==  83
	{80, 6},  // idx ==  84
	{84, 6},  // idx ==  85
	{88, 6},  // idx ==  86
	{92, 6},  // idx ==  87
	{96, 6},  // idx ==  88
	{100, 6}, // idx ==  89
	{104, 6}, // idx ==  90
	{108, 6}, // idx ==  91
	{112, 6}, // idx ==  92
	{116, 6}, // idx ==  93
	{120, 6}, // idx ==  94
	{124, 6}, // idx ==  95
	{128, 6}, // idx ==  96
	{132, 6}, // idx ==  97
	{136, 6}, // idx ==  98
	{140, 6}, // idx ==  99
	{144, 6}, // idx == 100
	{148, 6}, // idx == 101
	{152, 6}, // idx == 102
	{156, 6}, // idx == 103
	{160, 6}, // idx == 104
	{164, 6}, // idx == 105
	{168, 6}, // idx == 106
	{172, 6}, // idx == 107
	{176, 6}, // idx == 108
	{180, 6}, // idx == 109
	{184, 6}, // idx == 110
	{188, 6}, // idx == 111
	{192, 6}, // idx == 112
	{196, 6}, // idx == 113
	{200, 6}, // idx == 114
	{204, 6}, // idx == 115
	{208, 6}, // idx == 116
	{212, 6}, // idx == 117
	{216, 6}, // idx == 118
	{220, 6}, // idx == 119
	{224, 6}, // idx == 120
	{228, 6}, // idx == 121
	{232, 6}, // idx == 122
	{236, 6}, // idx == 123
	{240, 6}, // idx == 124
	{244, 6}, // idx == 125
	{248, 6}, // idx == 126
	{252, 6}, // idx == 127
	{0, 7},   // idx == 128
	{2, 7},   // idx == 129
	{4, 7},   // idx == 130
	{6, 7},   // idx == 131
	{8, 7},   // idx == 132
	{10, 7},  // idx == 133
	{12, 7},  // idx == 134
	{14, 7},  // idx == 135
	{16, 7},  // idx == 136
	{18, 7},  // idx == 137
	{20, 7},  // idx == 138
	{22, 7},  // idx == 139
	{24, 7},  // idx == 140
	{26, 7},  // idx == 141
	{28, 7},  // idx == 142
	{30, 7},  // idx == 143
	{32, 7},  // idx == 144
	{34, 7},  // idx == 145
	{36, 7},  // idx == 146
	{38, 7},  // idx == 147
	{40, 7},  // idx == 148
	{42, 7},  // idx == 149
	{44, 7},  // idx == 150
	{46, 7},  // idx == 151
	{48, 7},  // idx == 152
	{50, 7},  // idx == 153
	{52, 7},  // idx == 154
	{54, 7},  // idx == 155
	{56, 7},  // idx == 156
	{58, 7},  // idx == 157
	{60, 7},  // idx == 158
	{62, 7},  // idx == 159
	{64, 7},  // idx == 160
	{66, 7},  // idx == 161
	{68, 7},  // idx == 162
	{70, 7},  // idx == 163
	{72, 7},  // idx == 164
	{74, 7},  // idx == 165
	{76, 7},  // idx == 166
	{78, 7},  // idx == 167
	{80, 7},  // idx == 168
	{82, 7},  // idx == 169
	{84, 7},  // idx == 170
	{86, 7},  // idx == 171
	{88, 7},  // idx == 172
	{90, 7},  // idx == 173
	{92, 7},  // idx == 174
	{94, 7},  // idx == 175
	{96, 7},  // idx == 176
	{98, 7},  // idx == 177
	{100, 7}, // idx == 178
	{102, 7}, // idx == 179
	{104, 7}, // idx == 180
	{106, 7}, // idx == 181
	{108, 7}, // idx == 182
	{110, 7}, // idx == 183
	{112, 7}, // idx == 184
	{114, 7}, // idx == 185
	{116, 7}, // idx == 186
	{118, 7}, // idx == 187
	{120, 7}, // idx == 188
	{122, 7}, // idx == 189
	{124, 7}, // idx == 190
	{126, 7}, // idx == 191
	{128, 7}, // idx == 192
	{130, 7}, // idx == 193
	{132, 7}, // idx == 194
	{134, 7}, // idx == 195
	{136, 7}, // idx == 196
	{138, 7}, // idx == 197
	{140, 7}, // idx == 198
	{142, 7}, // idx == 199
	{144, 7}, // idx == 200
	{146, 7}, // idx == 201
	{148, 7}, // idx == 202
	{150, 7}, // idx == 203
	{152, 7}, // idx == 204
	{154, 7}, // idx == 205
	{156, 7}, // idx == 206
	{158, 7}, // idx == 207
	{160, 7}, // idx == 208
	{162, 7}, // idx == 209
	{164, 7}, // idx == 210
	{166, 7}, // idx == 211
	{168, 7}, // idx == 212
	{170, 7}, // idx == 213
	{172, 7}, // idx == 214
	{174, 7}, // idx == 215
	{176, 7}, // idx == 216
	{178, 7}, // idx == 217
	{180, 7}, // idx == 218
	{182, 7}, // idx == 219
	{184, 7}, // idx == 220
	{186, 7}, // idx == 221
	{188, 7}, // idx == 222
	{190, 7}, // idx == 223
	{192, 7}, // idx == 224
	{194, 7}, // idx == 225
	{196, 7}, // idx == 226
	{198, 7}, // idx == 227
	{200, 7}, // idx == 228
	{202, 7}, // idx == 229
	{204, 7}, // idx == 230
	{206, 7}, // idx == 231
	{208, 7}, // idx == 232
	{210, 7}, // idx == 233
	{212, 7}, // idx == 234
	{214, 7}, // idx == 235
	{216, 7}, // idx == 236
	{218, 7}, // idx == 237
	{220, 7}, // idx == 238
	{222, 7}, // idx == 239
	{224, 7}, // idx == 240
	{226, 7}, // idx == 241
	{228, 7}, // idx == 242
	{230, 7}, // idx == 243
	{232, 7}, // idx == 244
	{234, 7}, // idx == 245
	{236, 7}, // idx == 246
	{238, 7}, // idx == 247
	{240, 7}, // idx == 248
	{242, 7}, // idx == 249
	{244, 7}, // idx == 250
	{246, 7}, // idx == 251
	{248, 7}, // idx == 252
	{250, 7}, // idx == 253
	{252, 7}, // idx == 254
	{254, 7}, // idx == 255
	{0, 8},   // idx == 256 firstHostIndex
	{1, 8},   // idx == 257
	{2, 8},   // idx == 258
	{3, 8},   // idx == 259
	{4, 8},   // idx == 260
	{5, 8},   // idx == 261
	{6, 8},   // idx == 262
	{7, 8},   // idx == 263
	{8, 8},   // idx == 264
	{9, 8},   // idx == 265
	{10, 8},  // idx == 266
	{11, 8},  // idx == 267
	{12, 8},  // idx == 268
	{13, 8},  // idx == 269
	{14, 8},  // idx == 270
	{15, 8},  // idx == 271
	{16, 8},  // idx == 272
	{17, 8},  // idx == 273
	{18, 8},  // idx == 274
	{19, 8},  // idx == 275
	{20, 8},  // idx == 276
	{21, 8},  // idx == 277
	{22, 8},  // idx == 278
	{23, 8},  // idx == 279
	{24, 8},  // idx == 280
	{25, 8},  // idx == 281
	{26, 8},  // idx == 282
	{27, 8},  // idx == 283
	{28, 8},  // idx == 284
	{29, 8},  // idx == 285
	{30, 8},  // idx == 286
	{31, 8},  // idx == 287
	{32, 8},  // idx == 288
	{33, 8},  // idx == 289
	{34, 8},  // idx == 290
	{35, 8},  // idx == 291
	{36, 8},  // idx == 292
	{37, 8},  // idx == 293
	{38, 8},  // idx == 294
	{39, 8},  // idx == 295
	{40, 8},  // idx == 296
	{41, 8},  // idx == 297
	{42, 8},  // idx == 298
	{43, 8},  // idx == 299
	{44, 8},  // idx == 300
	{45, 8},  // idx == 301
	{46, 8},  // idx == 302
	{47, 8},  // idx == 303
	{48, 8},  // idx == 304
	{49, 8},  // idx == 305
	{50, 8},  // idx == 306
	{51, 8},  // idx == 307
	{52, 8},  // idx == 308
	{53, 8},  // idx == 309
	{54, 8},  // idx == 310
	{55, 8},  // idx == 311
	{56, 8},  // idx == 312
	{57, 8},  // idx == 313
	{58, 8},  // idx == 314
	{59, 8},  // idx == 315
	{60, 8},  // idx == 316
	{61, 8},  // idx == 317
	{62, 8},  // idx == 318
	{63, 8},  // idx == 319
	{64, 8},  // idx == 320
	{65, 8},  // idx == 321
	{66, 8},  // idx == 322
	{67, 8},  // idx == 323
	{68, 8},  // idx == 324
	{69, 8},  // idx == 325
	{70, 8},  // idx == 326
	{71, 8},  // idx == 327
	{72, 8},  // idx == 328
	{73, 8},  // idx == 329
	{74, 8},  // idx == 330
	{75, 8},  // idx == 331
	{76, 8},  // idx == 332
	{77, 8},  // idx == 333
	{78, 8},  // idx == 334
	{79, 8},  // idx == 335
	{80, 8},  // idx == 336
	{81, 8},  // idx == 337
	{82, 8},  // idx == 338
	{83, 8},  // idx == 339
	{84, 8},  // idx == 340
	{85, 8},  // idx == 341
	{86, 8},  // idx == 342
	{87, 8},  // idx == 343
	{88, 8},  // idx == 344
	{89, 8},  // idx == 345
	{90, 8},  // idx == 346
	{91, 8},  // idx == 347
	{92, 8},  // idx == 348
	{93, 8},  // idx == 349
	{94, 8},  // idx == 350
	{95, 8},  // idx == 351
	{96, 8},  // idx == 352
	{97, 8},  // idx == 353
	{98, 8},  // idx == 354
	{99, 8},  // idx == 355
	{100, 8}, // idx == 356
	{101, 8}, // idx == 357
	{102, 8}, // idx == 358
	{103, 8}, // idx == 359
	{104, 8}, // idx == 360
	{105, 8}, // idx == 361
	{106, 8}, // idx == 362
	{107, 8}, // idx == 363
	{108, 8}, // idx == 364
	{109, 8}, // idx == 365
	{110, 8}, // idx == 366
	{111, 8}, // idx == 367
	{112, 8}, // idx == 368
	{113, 8}, // idx == 369
	{114, 8}, // idx == 370
	{115, 8}, // idx == 371
	{116, 8}, // idx == 372
	{117, 8}, // idx == 373
	{118, 8}, // idx == 374
	{119, 8}, // idx == 375
	{120, 8}, // idx == 376
	{121, 8}, // idx == 377
	{122, 8}, // idx == 378
	{123, 8}, // idx == 379
	{124, 8}, // idx == 380
	{125, 8}, // idx == 381
	{126, 8}, // idx == 382
	{127, 8}, // idx == 383
	{128, 8}, // idx == 384
	{129, 8}, // idx == 385
	{130, 8}, // idx == 386
	{131, 8}, // idx == 387
	{132, 8}, // idx == 388
	{133, 8}, // idx == 389
	{134, 8}, // idx == 390
	{135, 8}, // idx == 391
	{136, 8}, // idx == 392
	{137, 8}, // idx == 393
	{138, 8}, // idx == 394
	{139, 8}, // idx == 395
	{140, 8}, // idx == 396
	{141, 8}, // idx == 397
	{142, 8}, // idx == 398
	{143, 8}, // idx == 399
	{144, 8}, // idx == 400
	{145, 8}, // idx == 401
	{146, 8}, // idx == 402
	{147, 8}, // idx == 403
	{148, 8}, // idx == 404
	{149, 8}, // idx == 405
	{150, 8}, // idx == 406
	{151, 8}, // idx == 407
	{152, 8}, // idx == 408
	{153, 8}, // idx == 409
	{154, 8}, // idx == 410
	{155, 8}, // idx == 411
	{156, 8}, // idx == 412
	{157, 8}, // idx == 413
	{158, 8}, // idx == 414
	{159, 8}, // idx == 415
	{160, 8}, // idx == 416
	{161, 8}, // idx == 417
	{162, 8}, // idx == 418
	{163, 8}, // idx == 419
	{164, 8}, // idx == 420
	{165, 8}, // idx == 421
	{166, 8}, // idx == 422
	{167, 8}, // idx == 423
	{168, 8}, // idx == 424
	{169, 8}, // idx == 425
	{170, 8}, // idx == 426
	{171, 8}, // idx == 427
	{172, 8}, // idx == 428
	{173, 8}, // idx == 429
	{174, 8}, // idx == 430
	{175, 8}, // idx == 431
	{176, 8}, // idx == 432
	{177, 8}, // idx == 433
	{178, 8}, // idx == 434
	{179, 8}, // idx == 435
	{180, 8}, // idx == 436
	{181, 8}, // idx == 437
	{182, 8}, // idx == 438
	{183, 8}, // idx == 439
	{184, 8}, // idx == 440
	{185, 8}, // idx == 441
	{186, 8}, // idx == 442
	{187, 8}, // idx == 443
	{188, 8}, // idx == 444
	{189, 8}, // idx == 445
	{190, 8}, // idx == 446
	{191, 8}, // idx == 447
	{192, 8}, // idx == 448
	{193, 8}, // idx == 449
	{194, 8}, // idx == 450
	{195, 8}, // idx == 451
	{196, 8}, // idx == 452
	{197, 8}, // idx == 453
	{198, 8}, // idx == 454
	{199, 8}, // idx == 455
	{200, 8}, // idx == 456
	{201, 8}, // idx == 457
	{202, 8}, // idx == 458
	{203, 8}, // idx == 459
	{204, 8}, // idx == 460
	{205, 8}, // idx == 461
	{206, 8}, // idx == 462
	{207, 8}, // idx == 463
	{208, 8}, // idx == 464
	{209, 8}, // idx == 465
	{210, 8}, // idx == 466
	{211, 8}, // idx == 467
	{212, 8}, // idx == 468
	{213, 8}, // idx == 469
	{214, 8}, // idx == 470
	{215, 8}, // idx == 471
	{216, 8}, // idx == 472
	{217, 8}, // idx == 473
	{218, 8}, // idx == 474
	{219, 8}, // idx == 475
	{220, 8}, // idx == 476
	{221, 8}, // idx == 477
	{222, 8}, // idx == 478
	{223, 8}, // idx == 479
	{224, 8}, // idx == 480
	{225, 8}, // idx == 481
	{226, 8}, // idx == 482
	{227, 8}, // idx == 483
	{228, 8}, // idx == 484
	{229, 8}, // idx == 485
	{230, 8}, // idx == 486
	{231, 8}, // idx == 487
	{232, 8}, // idx == 488
	{233, 8}, // idx == 489
	{234, 8}, // idx == 490
	{235, 8}, // idx == 491
	{236, 8}, // idx == 492
	{237, 8}, // idx == 493
	{238, 8}, // idx == 494
	{239, 8}, // idx == 495
	{240, 8}, // idx == 496
	{241, 8}, // idx == 497
	{242, 8}, // idx == 498
	{243, 8}, // idx == 499
	{244, 8}, // idx == 500
	{245, 8}, // idx == 501
	{246, 8}, // idx == 502
	{247, 8}, // idx == 503
	{248, 8}, // idx == 504
	{249, 8}, // idx == 505
	{250, 8}, // idx == 506
	{251, 8}, // idx == 507
	{252, 8}, // idx == 508
	{253, 8}, // idx == 509
	{254, 8}, // idx == 510
	{255, 8}, // idx == 511
}
